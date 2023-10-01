/*
 * Copyright (C) 2023 Cristian Magherusan-Stanciu. All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the Open Software License version 3.0 as published
 * by the Open Source Initiative.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * Open Software License version 3.0 for more details.
 *
 * You should have received a copy of the Open Software License version 3.0
 * along with this program. If not, see <https://opensource.org/licenses/OSL-3.0>.
 */

package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

type LoadBalancerInfo struct {
	Region          string
	Type            string
	DNSName         string
	IPCount         int
	TrafficLastWeek int
	PublicIPs       []string
	Cost            float64
}

func fetchLoadBalancers(client *elbv2.Client) ([]elbv2types.LoadBalancer, error) {
	debug.Printf("Fetching ALBs and NLBs...")
	resp, err := client.DescribeLoadBalancers(context.TODO(), &elbv2.DescribeLoadBalancersInput{})
	if err != nil {
		debug.Printf("Error fetching ALBs and NLBs: %v", err)
		return nil, err
	}
	debug.Printf("Fetched %d ALBs and NLBs.", len(resp.LoadBalancers))
	return resp.LoadBalancers, nil
}

func fetchClassicLoadBalancers(client *elb.Client) ([]elbtypes.LoadBalancerDescription, error) {
	debug.Printf("Fetching Classic ELBs...")
	resp, err := client.DescribeLoadBalancers(context.TODO(), &elb.DescribeLoadBalancersInput{})
	if err != nil {
		debug.Printf("Error fetching Classic ELBs: %v", err)
		return nil, err
	}
	debug.Printf("Fetched %d Classic ELBs.", len(resp.LoadBalancerDescriptions))
	return resp.LoadBalancerDescriptions, nil
}

func countIPsFromDNS(dnsName string) []string {
	debug.Printf("Resolving IPs for DNS name: %s", dnsName)
	ips, _ := net.LookupIP(dnsName)
	var ipStrings []string
	for _, ip := range ips {
		ipStrings = append(ipStrings, ip.String())
	}
	debug.Printf("Resolved %d IPs for DNS name: %s", len(ipStrings), dnsName)
	return ipStrings
}

func fetchProcessedBytes(lbIdentifier string, lbType string, cfg aws.Config) int {
	// Create a CloudWatch client
	cwClient := cloudwatch.NewFromConfig(cfg)

	// Determine the namespace and dimension based on the load balancer type
	var namespace, dimensionName string
	metricName := "ProcessedBytes"
	switch lbType {
	case "application":
		namespace = "AWS/ApplicationELB"
		dimensionName = "LoadBalancer"

	case "network":
		namespace = "AWS/NetworkELB"
		dimensionName = "LoadBalancer"

	case "classic":
		namespace = "AWS/ELB"
		dimensionName = "LoadBalancerName"
		metricName = "EstimatedProcessedBytes"

	default:
		return -1 // Return 0 for unsupported types
	}

	// Define the metric details
	metricDataQueries := []cwtypes.MetricDataQuery{
		{
			Id: aws.String("m1"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  aws.String(namespace),
					MetricName: aws.String(metricName),
					Dimensions: []cwtypes.Dimension{
						{
							Name:  aws.String(dimensionName),
							Value: aws.String(lbIdentifier),
						},
					},
				},
				Period: aws.Int32(3600), // 3600 seconds = 1 hour
				Stat:   aws.String("Sum"),
			},
			ReturnData: aws.Bool(true),
		},
	}

	// Fetch the metric data
	resp, err := cwClient.GetMetricData(context.TODO(), &cloudwatch.GetMetricDataInput{
		StartTime:         aws.Time(time.Now().Add(-7 * 24 * time.Hour)), // 7 days ago
		EndTime:           aws.Time(time.Now()),
		MetricDataQueries: metricDataQueries,
	})

	if err != nil {
		// Handle the error
		return 0
	}

	// Extract the total bytes processed from the response
	totalBytes := 0
	if len(resp.MetricDataResults) > 0 {
		for _, value := range resp.MetricDataResults[0].Values {
			totalBytes += int(value)
		}
	}

	return totalBytes
}

func fetchAllLoadBalancers(cfg aws.Config, regions []types.Region) ([]LoadBalancerInfo, error) {
	var allLBs []LoadBalancerInfo
	lbInfoCh := make(chan LoadBalancerInfo)
	errCh := make(chan error)

	var wg sync.WaitGroup

	for _, region := range regions {
		wg.Add(1)
		go func(region types.Region) {
			defer wg.Done()

			regionalClient := elbv2.NewFromConfig(cfg, func(o *elbv2.Options) {
				o.Region = *region.RegionName
			})
			regionalClassicClient := elb.NewFromConfig(cfg, func(o *elb.Options) {
				o.Region = *region.RegionName
			})

			lbs, err := fetchLoadBalancers(regionalClient)
			if err != nil {
				errCh <- fmt.Errorf("failed to fetch LoadBalancers in region %s: %v", *region.RegionName, err)
				return
			}
			classicLbs, err := fetchClassicLoadBalancers(regionalClassicClient)
			if err != nil {
				errCh <- fmt.Errorf("failed to fetch Classic LoadBalancers in region %s: %v", *region.RegionName, err)
				return
			}

			for _, lb := range lbs {
				wg.Add(1)
				go func(lb elbv2types.LoadBalancer) {
					defer wg.Done()
					ips := countIPsFromDNS(*lb.DNSName)

					// Extract the relevant part of the ARN for ALBs and NLBs
					lbIdentifier := *lb.LoadBalancerArn
					if lb.Type == elbv2types.LoadBalancerTypeEnumApplication || lb.Type == elbv2types.LoadBalancerTypeEnumNetwork {
						parts := strings.Split(lbIdentifier, "loadbalancer/")
						if len(parts) > 1 {
							lbIdentifier = parts[1]
						} else {
							debug.Printf("Invalid ARN format: %s", *lb.LoadBalancerArn)
						}
					}

					lbInfoCh <- LoadBalancerInfo{
						Region:          *region.RegionName,
						Type:            string(lb.Type),
						DNSName:         *lb.DNSName,
						IPCount:         len(ips),
						TrafficLastWeek: fetchProcessedBytes(lbIdentifier, string(lb.Type), cfg),
						PublicIPs:       ips,
						Cost:            3.65 * float64(len(ips)),
					}
				}(lb)
			}

			for _, lb := range classicLbs {
				wg.Add(1)
				go func(lb elbtypes.LoadBalancerDescription) {
					defer wg.Done()
					ips := countIPsFromDNS(*lb.DNSName)
					lbInfoCh <- LoadBalancerInfo{
						Region:          *region.RegionName,
						Type:            "classic",
						DNSName:         *lb.DNSName,
						IPCount:         len(ips),
						TrafficLastWeek: fetchProcessedBytes(*lb.LoadBalancerName, "classic", cfg),
						PublicIPs:       ips,
						Cost:            3.65 * float64(len(ips)),
					}
				}(lb)
			}
		}(region)
	}

	// Close channels once all goroutines are done
	go func() {
		wg.Wait()
		close(lbInfoCh)
		close(errCh)
	}()

	var errors []string
	// Collect results from the channels
	for lbInfo := range lbInfoCh {
		allLBs = append(allLBs, lbInfo)
	}
	for err := range errCh {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return allLBs, fmt.Errorf(strings.Join(errors, "; "))
	}
	return allLBs, nil
}
