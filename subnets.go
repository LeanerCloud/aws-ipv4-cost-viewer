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

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/rivo/tview"
)

func handleSubnets() {
	// Initialize AWS SDK
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Unable to load SDK config, %v", err)
	}
	client := ec2.NewFromConfig(cfg)

	// Fetch all regions
	regions, err := client.DescribeRegions(context.TODO(), &ec2.DescribeRegionsInput{})
	if err != nil {
		log.Fatalf("Failed to describe regions, %v", err)
	}

	// Create a tview table for display
	table := tview.NewTable().SetBorders(true)
	table.SetTitle("VPC Subnets").SetBorder(true)

	// Headers
	table.SetCell(0, 0, tview.NewTableCell("Region"))
	table.SetCell(0, 1, tview.NewTableCell("VPC ID"))
	table.SetCell(0, 2, tview.NewTableCell("Subnet ID"))
	table.SetCell(0, 3, tview.NewTableCell("Auto-Attach IP"))

	row := 1
	for _, region := range regions.Regions {
		// Create a regional client
		regionalClient := ec2.NewFromConfig(cfg, func(o *ec2.Options) {
			o.Region = *region.RegionName
		})

		// Fetch subnets in the region
		resp, err := regionalClient.DescribeSubnets(context.TODO(), &ec2.DescribeSubnetsInput{})
		if err != nil {
			log.Printf("Failed to describe subnets in region %s, %v", *region.RegionName, err)
			continue
		}

		// Populate the table with subnet data
		for _, subnet := range resp.Subnets {
			table.SetCell(row, 0, tview.NewTableCell(*region.RegionName))
			table.SetCell(row, 1, tview.NewTableCell(*subnet.VpcId))
			table.SetCell(row, 2, tview.NewTableCell(*subnet.SubnetId))
			table.SetCell(row, 3, tview.NewTableCell(fmt.Sprintf("%v", *subnet.MapPublicIpOnLaunch)))

			// Toggle the MapPublicIpOnLaunch attribute
			newValue := !*subnet.MapPublicIpOnLaunch
			_, err := regionalClient.ModifySubnetAttribute(context.TODO(), &ec2.ModifySubnetAttributeInput{
				SubnetId: subnet.SubnetId,
				MapPublicIpOnLaunch: &types.AttributeBooleanValue{
					Value: &newValue,
				},
			})
			if err != nil {
				log.Printf("Failed to toggle Auto-Attach IP for subnet %s, %v", *subnet.SubnetId, err)
			}
			row++
		}
	}

	// Create and configure the tview application
	app := tview.NewApplication()
	app.SetRoot(table, true).SetFocus(table)

	// Run the tview application
	if err := app.Run(); err != nil {
		log.Fatalf("Failed to run application: %v", err)
	}
}
