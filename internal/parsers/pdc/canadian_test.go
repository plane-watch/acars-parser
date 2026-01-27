package pdc

import (
	"fmt"
	"testing"
)

func TestCanadianNavPattern(t *testing.T) {
	text := `-// ATC PA01 YYZOWAC 03JAN/0637          C-FSIL/508/AC0348
TIMESTAMP 03JAN26 06:25
*PRE-DEPARTURE CLEARANCE*
FLT ACA348    CYVR 
M/B38M/W FILED FL350 
XPRD 0032 
 
USE SID FSR8
DEPARTURE RUNWAY 08R
DESTINATION CYOW
CONTACT CLEARANCE DELIVERY 121.4 WITH
IDENTIFIER 585U
 
ALNOD IKNIX AXILI 4930N10000W
AGLIN SMARE MEECH4
END`

	compiler := NewCompiler()
	if err := compiler.Compile(); err != nil {
		t.Fatalf("Failed to compile: %v", err)
	}

	result := compiler.Parse(text)
	if result == nil {
		t.Fatal("Expected match, got nil")
	}

	fmt.Printf("Format: %s\n", result.FormatName)
	fmt.Printf("Flight: %s\n", result.FlightNumber)
	fmt.Printf("Origin: %s\n", result.Origin)
	fmt.Printf("Destination: %s\n", result.Destination)
	fmt.Printf("Runway: %s\n", result.Runway)
	fmt.Printf("SID: %s\n", result.SID)
	fmt.Printf("Altitude: %s\n", result.Altitude)

	if result.FormatName != "canadian_nav" {
		t.Errorf("Expected format canadian_nav, got %s", result.FormatName)
	}
	if result.Origin != "CYVR" {
		t.Errorf("Expected origin CYVR, got %s", result.Origin)
	}
	if result.Destination != "CYOW" {
		t.Errorf("Expected destination CYOW, got %s", result.Destination)
	}
	// Verify we're NOT extracting FSIL as origin
	if result.Origin == "FSIL" {
		t.Error("FSIL should NOT be extracted as origin - it's the aircraft registration!")
	}
}

func TestUSRegionalPattern(t *testing.T) {
	text := `PDC
001
PDT5898 1772 KPHL
E145/L P1834
145 310
-DITCH T416 JIMEE-
KPHL DITCH V312 JIMEE WAVEY`

	compiler := NewCompiler()
	if err := compiler.Compile(); err != nil {
		t.Fatalf("Failed to compile: %v", err)
	}

	result := compiler.Parse(text)
	if result == nil {
		t.Fatal("Expected match, got nil")
	}

	fmt.Printf("\nUS Regional:\n")
	fmt.Printf("Format: %s\n", result.FormatName)
	fmt.Printf("Flight: %s\n", result.FlightNumber)
	fmt.Printf("Origin: %s\n", result.Origin)
	fmt.Printf("Squawk: %s\n", result.Squawk)
}
