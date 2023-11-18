package main

import (
    "encoding/csv"
    "fmt"
    "os"
)

// ExportToCSV exports EC2 instance data to a CSV file
func ExportToCSV(instances []EC2InstanceInfo, filename string) error {
    file, err := os.Create(filename)
    if err != nil {
        return fmt.Errorf("failed to create file: %v", err)
    }
    defer file.Close()

    writer := csv.NewWriter(file)
    defer writer.Flush()

    // Write CSV headers
    headers := []string{"Region", "Name Tag", "Instance State", "Instance ID", "Public IP", "VPC ID", "Subnet ID", "Cost"}
    if err := writer.Write(headers); err != nil {
        return fmt.Errorf("failed to write headers: %v", err)
    }

    // Write instance data to CSV
    for _, instance := range instances {
        record := []string{
            instance.Region,
            instance.NameTag,
            instance.InstanceState,
            instance.InstanceID,
            instance.PublicIP,
            instance.VPCID,
            instance.SubnetID,
            fmt.Sprintf("%.2f", instance.Cost),
        }
        if err := writer.Write(record); err != nil {
            return fmt.Errorf("failed to write record: %v", err)
        }
    }

    return nil
}



