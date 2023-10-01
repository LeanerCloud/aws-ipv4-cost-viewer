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
	"log"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type EC2InstanceInfo struct {
	Region        string
	NameTag       string
	InstanceState string
	InstanceID    string
	PublicIP      string
	VPCID         string
	SubnetID      string
	Cost          float64
}

func fetchInstancesInRegion(conf aws.Config, regionName string) ([]types.Instance, error) {
	// Create a regional client
	regionalClient := ec2.NewFromConfig(conf, func(o *ec2.Options) {
		o.Region = regionName
	})

	// Fetch instances in the region
	resp, err := regionalClient.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances in region %s: %v", regionName, err)
	}

	var filteredInstances []types.Instance
	for _, reservation := range resp.Reservations {
		for _, instance := range reservation.Instances {
			if instance.PublicIpAddress != nil && *instance.PublicIpAddress != "" {
				filteredInstances = append(filteredInstances, instance)
			}
		}
	}
	return filteredInstances, nil
}
func fetchAllInstances(config aws.Config, regions []types.Region) ([]EC2InstanceInfo, error) {
	var allInstances []EC2InstanceInfo
	var mu sync.Mutex
	var wg sync.WaitGroup

	debug.Println("Starting fetchAllInstances...")

	for _, region := range regions {
		wg.Add(1)
		go func(region types.Region) {
			defer wg.Done()

			debug.Printf("Fetching instances for region: %s", *region.RegionName)

			instances, err := fetchInstancesInRegion(config, *region.RegionName)
			if err != nil {
				log.Printf("Failed to fetch instances in region %s: %v", *region.RegionName, err)
				return
			}

			debug.Printf("Fetched %d instances for region %s", len(instances), *region.RegionName)

			for _, instance := range instances {
				nameTag := getNameTagValue(instance.Tags)
				inst := EC2InstanceInfo{
					Region:        *region.RegionName,
					NameTag:       nameTag,
					InstanceState: string(instance.State.Name),
					InstanceID:    *instance.InstanceId,
					PublicIP:      *instance.PublicIpAddress,
					VPCID:         *instance.VpcId,
					SubnetID:      *instance.SubnetId,
					Cost:          3.65,
				}
				mu.Lock()
				allInstances = append(allInstances, inst)
				mu.Unlock()
			}

		}(region)
	}

	wg.Wait()

	debug.Printf("Finished fetchAllInstances. Total instances fetched: %d", len(allInstances))

	return allInstances, nil
}
