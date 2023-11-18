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
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

var debug *log.Logger

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	debugEnv := os.Getenv("DEBUG")
	if debugEnv == "true" {
		debug = log.New(os.Stdout, "[DEBUG] ", log.LstdFlags|log.Lshortfile)
	} else {
		debug = log.New(io.Discard, "", 0) // No-op logger
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--subnets" {
		handleSubnets()
	} else {
		ipCostsView()
	}
}

func fetchRegions(client *ec2.Client) ([]types.Region, error) {
	regions, err := client.DescribeRegions(context.TODO(), &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, err
	}
	return regions.Regions, nil
}




