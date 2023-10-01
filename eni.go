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

type ENIInfo struct {
	Region   string
	PublicIP string
	ENIID    string
	Cost     float64
}

func fetchENIsInRegion(conf aws.Config, regionName string) ([]types.NetworkInterface, error) {
	regionalClient := ec2.NewFromConfig(conf, func(o *ec2.Options) {
		o.Region = regionName
	})

	resp, err := regionalClient.DescribeNetworkInterfaces(context.TODO(), &ec2.DescribeNetworkInterfacesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe ENIs in region %s: %v", regionName, err)
	}

	var filteredENIs []types.NetworkInterface
	for _, eni := range resp.NetworkInterfaces {
		if eni.Association != nil && eni.Association.PublicIp != nil && *eni.Association.PublicIp != "" {
			filteredENIs = append(filteredENIs, eni)
		}
	}
	return filteredENIs, nil
}

func fetchAllENIs(config aws.Config, regions []types.Region) ([]ENIInfo, error) {
	var allENIs []ENIInfo
	var errors []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	eniCh := make(chan ENIInfo, len(regions)*10) // Assuming a max of 10 ENIs per region
	errCh := make(chan string, len(regions))

	for _, region := range regions {
		wg.Add(1)
		go func(region types.Region) {
			defer wg.Done()

			enis, err := fetchENIsInRegion(config, *region.RegionName)
			if err != nil {
				errCh <- fmt.Sprintf("Failed to fetch ENIs in region %s: %v", *region.RegionName, err)
				return
			}

			for _, eni := range enis {
				eniCh <- ENIInfo{
					Region:   *region.RegionName,
					PublicIP: *eni.Association.PublicIp,
					ENIID:    *eni.NetworkInterfaceId,
					Cost:     3.65,
				}
			}
		}(region)
	}

	go func() {
		wg.Wait()
		close(eniCh)
		close(errCh)
	}()

	for eni := range eniCh {
		mu.Lock()
		allENIs = append(allENIs, eni)
		mu.Unlock()
	}

	for err := range errCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return allENIs, fmt.Errorf(strings.Join(errors, "; "))
	}
	return allENIs, nil
}
