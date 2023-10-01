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

import "github.com/aws/aws-sdk-go-v2/service/ec2/types"

func getTagValue(tags []types.Tag, key string) string {
	for _, tag := range tags {
		if *tag.Key == key {
			return *tag.Value
		}
	}
	return ""
}

func getNameTagValue(tags []types.Tag) string {
	nameTag := getTagValue(tags, "Name")
	if nameTag == "" {
		nameTag = getTagValue(tags, "aws:cloudformation:stack-name")
	}
	if nameTag == "" {
		nameTag = getTagValue(tags, "aws:autoscaling:groupName")
	}
	return nameTag
}
