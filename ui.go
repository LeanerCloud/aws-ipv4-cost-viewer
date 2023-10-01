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
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	EIPCostPerHour     = 0.005
	HoursInMonth       = 720
	FlatFeePerPublicIP = 3.65
	TimeoutForEC2      = 20 * time.Second
	TimeoutForLB       = 20 * time.Second
	TimeoutForEIP      = 20 * time.Second
	TimeoutForENI      = 20 * time.Second
)

type ChannelData struct {
	table *tview.Table
	count int
	cost  float64
	err   error
}

func ipCostsView() error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %v", err)
	}

	ec2Client := ec2.NewFromConfig(cfg)
	regions, err := fetchRegions(ec2Client)
	if err != nil {
		log.Fatalf("Failed to fetch regions: %v", err)
	}

	ec2Ch, lbCh, eipCh, eniCh := make(chan ChannelData), make(chan ChannelData), make(chan ChannelData), make(chan ChannelData)

	go func() {
		defer close(ec2Ch)
		fetchTableData(createAndPopulateInstancesTable, cfg, regions, ec2Ch)
		debug.Printf("Finished fetching instances table data")
	}()

	go func() {
		defer close(lbCh)
		fetchTableData(createAndPopulateLBTable, cfg, regions, lbCh)
		debug.Printf("Finished fetching LB table data")
	}()

	go func() {
		defer close(eipCh)
		fetchTableData(createAndPopulateEIPsTable, cfg, regions, eipCh)
		debug.Printf("Finished fetching EIPs table data")
	}()

	go func() {
		defer close(eniCh)
		fetchTableData(createAndPopulateENIsTable, cfg, regions, eniCh)
		debug.Printf("Finished fetching ENIs table data")
	}()

	err = runUI(ec2Ch, lbCh, eipCh, eniCh)
	if err != nil {
		return err
	}

	return nil
}

func fetchTableData(fetchFunc func(aws.Config, []types.Region) (*tview.Table, int, float64, error),
	cfg aws.Config,
	regions []types.Region,
	ch chan ChannelData) {

	debug.Println("Starting data fetch...")
	startTime := time.Now()
	table, count, cost, err := fetchFunc(cfg, regions)
	debug.Printf("Data fetch completed in %v seconds", time.Since(startTime).Seconds())

	if err != nil {
		log.Printf("Error fetching table data: %v", err)
		ch <- ChannelData{nil, 0, 0, err}
		return
	}
	ch <- ChannelData{table, count, cost, nil}
}

func createLoadingView() *tview.TextView {
	return tview.NewTextView().SetText("Loading...").SetTextAlign(tview.AlignCenter)
}

func createTabs(tables []*tview.Table) (*tview.Pages, *tview.TextView) {
	pageOrder := []string{"Elastic Network Interfaces (also include EC2, LBs amd EIPs)",
		"EC2 Instances (includes attached EIPs)",
		"Load Balancers",
		"EIPs not attached to instances"}

	tabs := tview.NewPages()
	for i, table := range tables {
		tabs.AddPage(pageOrder[i], table, true, false)
	}

	tabNames := tview.NewTextView()
	currentIndex := 0

	updateTabNames := func() {
		tabText := ""
		for i, name := range pageOrder {
			if i == currentIndex {
				tabText += fmt.Sprintf("[::b][#0000ff]%s[white::-] | ", name) // Bold and red for active tab
			} else {
				tabText += fmt.Sprintf("%s | ", name) // Regular for inactive tabs
			}
		}
		tabText = strings.TrimSuffix(tabText, " | ")
		tabNames.SetText(tabText).SetDynamicColors(true)
	}

	tabs.SwitchToPage(pageOrder[0])
	updateTabNames()

	tabs.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRight:
			currentIndex = (currentIndex + 1) % len(pageOrder)
			tabs.SwitchToPage(pageOrder[currentIndex])
			updateTabNames()
		case tcell.KeyLeft:
			currentIndex = (currentIndex - 1 + len(pageOrder)) % len(pageOrder)
			tabs.SwitchToPage(pageOrder[currentIndex])
			updateTabNames()
		}
		return event
	})

	return tabs, tabNames
}

func createMainLayout(tabs *tview.Pages, tabNames *tview.TextView, counts []int, costs []float64) (*tview.Flex, []*tview.TextView) {
	costSummaries := []string{
		"--------------------------------",
		fmt.Sprintf("Public IPs attached to %d Elastic Network Intefaces: $%.2f", counts[0], costs[0]),
		fmt.Sprintf("EC2: $%.2f for %d instances", costs[1], counts[1]),
		fmt.Sprintf("Load balancers: $%.2f for %d load balancer IPs", costs[2], counts[2]),
		fmt.Sprintf("and $%.2f for %d Elastic IPs", costs[3], counts[3]),
		"Note: ENI costs also include those for EC2, LB and EIP. Still, unattached EIPs have an additional cost, so the total IPv4 cost isn't exactly the same as the ENI cost",
		"--------------------------------",
	}

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tabNames, 1, 0, false).
		AddItem(tabs, 0, 1, true)

	var costTextViews []*tview.TextView
	for _, summary := range costSummaries {
		tv := tview.NewTextView().SetText(summary)
		flex.AddItem(tv, 1, 0, false)
		costTextViews = append(costTextViews, tv)
	}

	keyboardShortcuts := tview.NewTextView().SetText("Use arrows to move around | Press ESC to exit")
	flex.AddItem(keyboardShortcuts, 1, 0, false)

	return flex, costTextViews
}

func runUI(ec2Ch, lbCh, eipCh, eniCh chan ChannelData) error {
	app := tview.NewApplication()
	loadingView := createLoadingView()
	app.SetRoot(loadingView, true)

	go func() {
		tables, counts, costs, err := unpackChannelData(ec2Ch, lbCh, eipCh, eniCh)
		if err != nil {
			log.Fatalf("Error fetching data: %v", err)
		}

		tabs, tabNames := createTabs(tables)
		flex, _ := createMainLayout(tabs, tabNames, counts, costs)

		app.QueueUpdateDraw(func() {
			app.SetRoot(flex, true).SetFocus(tabs)
		})
	}()

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			app.Stop()
		}
		return event
	})

	if err := app.Run(); err != nil {
		return fmt.Errorf("failed to run application: %v", err)
	}

	return nil
}

func unpackChannelData(ec2Ch, lbCh, eipCh, eniCh chan ChannelData) ([]*tview.Table, []int, []float64, error) {
	log.Println("Unpacking channel data...")

	var eniData, ec2Data, lbData, eipData ChannelData

	channels := []struct {
		ch      chan ChannelData
		data    *ChannelData
		timeout time.Duration
	}{
		{eniCh, &eniData, TimeoutForENI},
		{ec2Ch, &ec2Data, TimeoutForEC2},
		{lbCh, &lbData, TimeoutForLB},
		{eipCh, &eipData, TimeoutForEIP},
	}

	for _, ch := range channels {
		select {
		case data, ok := <-ch.ch:
			if !ok {
				return nil, nil, nil, fmt.Errorf("channel was closed before data was received")
			}
			*ch.data = data
		case <-time.After(ch.timeout):
			return nil, nil, nil, fmt.Errorf("timeout waiting for data from %v channel", ch.ch)
		}

		if ch.data.err != nil {
			return nil, nil, nil, ch.data.err
		}
	}

	tables := []*tview.Table{eniData.table, ec2Data.table, lbData.table, eipData.table}
	counts := []int{eniData.count, ec2Data.count, lbData.count, eipData.count}
	costs := []float64{eniData.cost, ec2Data.cost, lbData.cost, eipData.cost}

	return tables, counts, costs, nil
}

// Helper function to set up the table
func setupTable(title string) *tview.Table {
	table := tview.NewTable().SetBorders(true)
	table.SetTitle(title).SetBorder(true)
	table.SetSelectable(true, false)
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return handleTableInput(table, event)
	})
	return table
}

// Helper function to handle table input
func handleTableInput(table *tview.Table, event *tcell.EventKey) *tcell.EventKey {
	row, _ := table.GetSelection()

	switch event.Key() {
	case tcell.KeyUp:
		if row <= 0 {
			return nil
		}
	case tcell.KeyDown:
		if row >= table.GetRowCount()-1 {
			return nil
		}
	}
	return event
}

// Helper function to set headers
func setTableHeaders(table *tview.Table, headers ...string) {
	for i, header := range headers {
		table.SetCell(0, i, tview.NewTableCell(header))
	}
}

func createAndPopulateInstancesTable(config aws.Config, regions []types.Region) (*tview.Table, int, float64, error) {
	debug.Println("Starting createAndPopulateInstancesTable...")

	table := setupTable("EC2 Instances costs")
	debug.Println("Table setup done.")

	setTableHeaders(table, "Region", "Name Tag", "Instance State", "Instance ID", "Public IP", "VPC ID", "Subnet ID", "Cost")
	debug.Println("Table headers set.")

	debug.Println("Fetching all EC2 instances...")
	allInstances, err := fetchAllInstances(config, regions)
	if err != nil {
		debug.Printf("Error fetching all EC2 instances: %v", err)
		return nil, 0, 0, err
	}
	debug.Printf("Fetched %d EC2 instances.", len(allInstances))

	debug.Println("Sorting instances by IP...")
	sortStructsByIP(allInstances, func(i int) string {
		return allInstances[i].PublicIP
	})
	debug.Println("Sorting done.")

	debug.Println("Populating table with instance data...")
	row := 1
	totalCost := 0.0
	for _, instanceInfo := range allInstances {
		table.SetCell(row, 0, tview.NewTableCell(instanceInfo.Region))
		table.SetCell(row, 1, tview.NewTableCell(instanceInfo.NameTag))
		table.SetCell(row, 2, tview.NewTableCell(instanceInfo.InstanceState))
		table.SetCell(row, 3, tview.NewTableCell(instanceInfo.InstanceID))
		table.SetCell(row, 4, tview.NewTableCell(instanceInfo.PublicIP))
		table.SetCell(row, 5, tview.NewTableCell(instanceInfo.VPCID))
		table.SetCell(row, 6, tview.NewTableCell(instanceInfo.SubnetID))
		table.SetCell(row, 7, tview.NewTableCell(fmt.Sprintf("%.2f", instanceInfo.Cost)))
		totalCost += instanceInfo.Cost
		row++
	}
	debug.Println("Instances table population done.")

	debug.Printf("Total Instances IPs cost: $%.2f", totalCost)

	debug.Println("Finished createAndPopulateInstancesTable.")
	return table, len(allInstances), totalCost, nil
}

func createAndPopulateEIPsTable(config aws.Config, regions []types.Region) (*tview.Table, int, float64, error) {
	debug.Println("Starting createAndPopulateEIPsTable...")

	table := setupTable("Elastic IPs")
	setTableHeaders(table, "Region", "Name tag", "Public IP", "Attached Resource", "Cost")

	debug.Println("Fetching all EIPs...")
	allEIPs, err := fetchAllEIPs(config, regions)
	if err != nil {
		debug.Printf("Error fetching all EIPs: %v", err)
		return nil, 0, 0, err
	}
	debug.Printf("Fetched %d EIPs", len(allEIPs))

	debug.Println("Sorting EIPs by IP...")
	sortStructsByIP(allEIPs, func(i int) string {
		return allEIPs[i].PublicIP
	})

	row := 1

	totalCost := 0.0
	debug.Println("Populating table with EIP data...")
	for _, eipInfo := range allEIPs {
		table.SetCell(row, 0, tview.NewTableCell(eipInfo.Region))
		table.SetCell(row, 1, tview.NewTableCell(eipInfo.NameTag))
		table.SetCell(row, 2, tview.NewTableCell(eipInfo.PublicIP))
		table.SetCell(row, 3, tview.NewTableCell(eipInfo.AssociationTarget))
		table.SetCell(row, 4, tview.NewTableCell(fmt.Sprintf("%.2f", eipInfo.Cost)))

		totalCost += eipInfo.Cost
		row++
	}

	debug.Printf("Finished createAndPopulateEIPsTable. Total EIPs: %d, Total Cost: %f", len(allEIPs), totalCost)
	return table, len(allEIPs), totalCost, nil
}

func createAndPopulateENIsTable(config aws.Config, regions []types.Region) (*tview.Table, int, float64, error) {
	debug.Println("Starting createAndPopulateENIsTable...")

	table := setupTable("Elastic Network Interfaces with Public IPs")
	setTableHeaders(table, "Region", "Public IP", "ENI ID", "Cost")

	debug.Println("Fetching all ENIs...")
	allENIs, err := fetchAllENIs(config, regions)
	if err != nil {
		debug.Printf("Error fetching all ENIs: %v", err)
		return nil, 0, 0, err
	}
	debug.Printf("Fetched %d ENIs", len(allENIs))

	debug.Println("Sorting ENIs by IP...")
	sortStructsByIP(allENIs, func(i int) string {
		return allENIs[i].PublicIP
	})

	debug.Println("Populating table with ENI data...")
	row := 1
	totalCost := 0.0
	for _, eniInfo := range allENIs {
		table.SetCell(row, 0, tview.NewTableCell(eniInfo.Region))
		table.SetCell(row, 1, tview.NewTableCell(eniInfo.PublicIP))
		table.SetCell(row, 2, tview.NewTableCell(eniInfo.ENIID))
		table.SetCell(row, 3, tview.NewTableCell(fmt.Sprintf("%.2f", eniInfo.Cost)))
		totalCost += eniInfo.Cost
		row++
	}

	debug.Printf("Finished createAndPopulateENIsTable. Total ENIs: %d, Total Cost: %f", len(allENIs), totalCost)
	return table, len(allENIs), totalCost, nil
}

func createAndPopulateLBTable(cfg aws.Config, regions []types.Region) (*tview.Table, int, float64, error) {
	debug.Println("Starting createAndPopulateLBTable...")

	table := setupTable("Load balancer costs")
	setTableHeaders(table, "Region", "Load Balancer Type", "DNS Name", "IP Count", "Traffic MBs (last 7 days)", "Cost")

	debug.Println("Fetching all load balancers...")
	allLBs, err := fetchAllLoadBalancers(cfg, regions)
	if err != nil {
		debug.Printf("Error fetching all load balancers: %v", err)
		return nil, 0, 0, err
	}
	debug.Printf("Fetched %d load balancers", len(allLBs))

	debug.Println("Sorting load balancers by IP...")
	sortStructsByIP(allLBs, func(i int) string {
		if len(allLBs[i].PublicIPs) > 0 {
			return allLBs[i].PublicIPs[0]
		}
		return "" // Default value if no IPs
	})

	row := 1
	totalIPCount := 0
	totalCost := 0.0
	debug.Println("Populating table with load balancer data...")
	for _, lbInfo := range allLBs {
		table.SetCell(row, 0, tview.NewTableCell(lbInfo.Region))
		table.SetCell(row, 1, tview.NewTableCell(lbInfo.Type))
		table.SetCell(row, 2, tview.NewTableCell(lbInfo.DNSName))
		table.SetCell(row, 3, tview.NewTableCell(strconv.Itoa(lbInfo.IPCount)))
		table.SetCell(row, 4, tview.NewTableCell(fmt.Sprintf("%.2f", float64(lbInfo.TrafficLastWeek)/1024.0/1024.0)))
		table.SetCell(row, 5, tview.NewTableCell(fmt.Sprintf("%.2f", lbInfo.Cost)))
		row++

		totalIPCount += lbInfo.IPCount
		totalCost += lbInfo.Cost
	}

	debug.Printf("Finished createAndPopulateLBTable. Total IP Count: %d, Total Cost: %f", totalIPCount, totalCost)
	return table, totalIPCount, totalCost, nil
}

func sortStructsByIP(data interface{}, getIP func(i int) string) {
	sort.Slice(data, func(i, j int) bool {
		ip1, ip2 := net.ParseIP(getIP(i)), net.ParseIP(getIP(j))
		if ip1 == nil || ip2 == nil {
			return false
		}
		return bytes.Compare(ip1, ip2) < 0
	})
}
