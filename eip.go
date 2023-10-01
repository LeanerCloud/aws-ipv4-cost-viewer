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
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type EIPInfo struct {
	Region            string
	PublicIP          string
	AssociationTarget string
	NameTag           string
	Cost              float64
}

const (
	FilterNameAssociationID   = "association-id"
	FilterNameAllocationID    = "allocation-id"
	AssociationTypeInstance   = "Instance"
	AssociationTypeNATGateway = "NAT Gateway"
)

func fetchEIPsInRegion(conf aws.Config, regionName string) ([]types.Address, error) {
	regionalClient := ec2.NewFromConfig(conf, func(o *ec2.Options) {
		o.Region = regionName
	})

	resp, err := regionalClient.DescribeAddresses(context.TODO(), &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe EIPs in region %s: %v", regionName, err)
	}

	return resp.Addresses, nil
}

func describeEIPByAssociationID(conf aws.Config, associationID string, regionName string) (string, error) {
	regionalClient := ec2.NewFromConfig(conf, func(o *ec2.Options) {
		o.Region = regionName
	})

	// Describe the EIP by association ID
	input := &ec2.DescribeAddressesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String(FilterNameAssociationID),
				Values: []string{associationID},
			},
		},
	}

	resp, err := regionalClient.DescribeAddresses(context.TODO(), input)
	if err != nil {
		debug.Printf("Error describing EIP with association ID %s: %v", associationID, err)
		return "", err
	}

	if len(resp.Addresses) == 0 {
		debug.Printf("No EIP found with association ID %s", associationID)
		return "", nil
	}

	if resp.Addresses[0].InstanceId != nil {
		return AssociationTypeInstance + ": " + aws.ToString(resp.Addresses[0].InstanceId), nil
	}

	if resp.Addresses[0].NetworkInterfaceId != nil {
		natResp, natErr := regionalClient.DescribeNatGateways(context.TODO(), &ec2.DescribeNatGatewaysInput{})
		if natErr != nil {
			debug.Printf("Error describing NAT Gateway with allocation ID %s: %v", aws.ToString(resp.Addresses[0].AllocationId), natErr)
			return "", natErr
		}

		for _, natGateway := range natResp.NatGateways {
			for _, natAddress := range natGateway.NatGatewayAddresses {
				if natAddress.AllocationId != nil && *natAddress.AllocationId == aws.ToString(resp.Addresses[0].AllocationId) {
					return AssociationTypeNATGateway + ": " + aws.ToString(natGateway.NatGatewayId), nil
				}
			}
		}
	}

	debug.Printf("EIP with association ID %s is not associated with any known targets", associationID)
	return "", nil
}

func fetchAllEIPs(config aws.Config, regions []types.Region) ([]EIPInfo, error) {
	var allEIPs []EIPInfo
	var errors []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	eipCh := make(chan EIPInfo, len(regions)*10)
	errCh := make(chan string, len(regions))

	for _, region := range regions {
		wg.Add(1)
		go func(region types.Region) {
			defer wg.Done()

			eips, err := fetchEIPsInRegion(config, *region.RegionName)
			if err != nil {
				errCh <- fmt.Sprintf("failed to fetch EIPs in region %s: %v", *region.RegionName, err)
				return
			}

			for _, eip := range eips {
				if eip.InstanceId != nil {
					continue
				}

				nameTag := getNameTagValue(eip.Tags)
				associationTarget, err := describeEIPByAssociationID(config, aws.ToString(eip.AssociationId), *region.RegionName)
				if err != nil {
					errCh <- fmt.Sprintf("failed to describe EIP associations in region %s: %v", *region.RegionName, err)
					return
				}
				eipInfo := EIPInfo{
					Region:            *region.RegionName,
					PublicIP:          *eip.PublicIp,
					AssociationTarget: associationTarget,
					NameTag:           nameTag,
					Cost:              3.65,
				}
				if associationTarget == "" {
					eipInfo.Cost += 3.65
				}

				eipCh <- eipInfo
			}
		}(region)
	}

	go func() {
		wg.Wait()
		close(eipCh)
		close(errCh)
	}()

	for eip := range eipCh {
		mu.Lock()
		allEIPs = append(allEIPs, eip)
		mu.Unlock()
	}

	for err := range errCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		debug.Printf("Encountered errors: %v", errors)
		return allEIPs, fmt.Errorf(strings.Join(errors, "; "))
	}
	return allEIPs, nil
}
