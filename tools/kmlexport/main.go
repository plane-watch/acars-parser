// Package main provides a tool to export waypoints from the PostgreSQL database to KML format.
// KML (Keyhole Markup Language) files can be viewed in Google Earth, Google Maps, and
// other mapping applications.
package main

import (
	"context"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"time"

	"acars_parser/internal/storage"
)

// KML structures for XML marshalling.
// These follow the KML 2.2 specification: https://developers.google.com/kml/documentation/kmlreference

// KML is the root element of a KML document.
type KML struct {
	XMLName   xml.Name `xml:"kml"`
	Namespace string   `xml:"xmlns,attr"`
	Document  Document `xml:"Document"`
}

// Document contains the document metadata and features.
type Document struct {
	Name        string      `xml:"name"`
	Description string      `xml:"description,omitempty"`
	Styles      []Style     `xml:"Style,omitempty"`
	Placemarks  []Placemark `xml:"Placemark"`
}

// Style defines the visual appearance of features.
type Style struct {
	ID        string    `xml:"id,attr"`
	IconStyle IconStyle `xml:"IconStyle"`
}

// IconStyle defines how icons are displayed.
type IconStyle struct {
	Scale float64 `xml:"scale,omitempty"`
	Icon  Icon    `xml:"Icon"`
}

// Icon specifies the icon image.
type Icon struct {
	Href string `xml:"href"`
}

// Placemark represents a geographic feature with geometry and metadata.
type Placemark struct {
	Name         string        `xml:"name"`
	Description  string        `xml:"description,omitempty"`
	StyleURL     string        `xml:"styleUrl,omitempty"`
	Point        Point         `xml:"Point"`
	ExtendedData *ExtendedData `xml:"ExtendedData,omitempty"`
}

// Point represents a geographic location.
type Point struct {
	Coordinates string `xml:"coordinates"` // Format: lon,lat,altitude
}

// ExtendedData holds custom data associated with a placemark.
type ExtendedData struct {
	Data []Data `xml:"Data"`
}

// Data represents a single piece of extended data.
type Data struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value"`
}

func main() {
	// PostgreSQL connection flags.
	pgHost := flag.String("pg-host", "localhost", "PostgreSQL host")
	pgPort := flag.Int("pg-port", 5432, "PostgreSQL port")
	pgUser := flag.String("pg-user", "acars", "PostgreSQL user")
	pgPassword := flag.String("pg-password", "", "PostgreSQL password")
	pgDB := flag.String("pg-db", "acars", "PostgreSQL database")

	output := flag.String("output", "", "Output KML file (default: stdout)")
	minSources := flag.Int("min-sources", 1, "Minimum source count to include a waypoint")
	showStats := flag.Bool("stats", false, "Show statistics only, don't export")
	verbose := flag.Bool("v", false, "Verbose output")

	flag.Parse()

	ctx := context.Background()

	pg, err := storage.OpenPostgres(ctx, storage.PostgresConfig{
		Host:     *pgHost,
		Port:     *pgPort,
		Database: *pgDB,
		User:     *pgUser,
		Password: *pgPassword,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening PostgreSQL: %v\n", err)
		os.Exit(1)
	}
	defer pg.Close()

	// Show stats mode.
	if *showStats {
		showWaypointStats(ctx, pg)
		return
	}

	// Query waypoints.
	waypoints, err := pg.ListWaypoints(ctx, *minSources)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying waypoints: %v\n", err)
		os.Exit(1)
	}

	if len(waypoints) == 0 {
		fmt.Fprintf(os.Stderr, "No waypoints found matching criteria\n")
		os.Exit(0)
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Exporting %d waypoints to KML\n", len(waypoints))
	}

	// Generate KML.
	kml := generateKML(waypoints)

	// Marshal to XML.
	xmlData, err := xml.MarshalIndent(kml, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating KML: %v\n", err)
		os.Exit(1)
	}

	// Add XML header.
	xmlOutput := xml.Header + string(xmlData)

	// Write output.
	if *output != "" {
		if err := os.WriteFile(*output, []byte(xmlOutput), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}
		if *verbose {
			fmt.Fprintf(os.Stderr, "Wrote %s\n", *output)
		}
	} else {
		fmt.Println(xmlOutput)
	}
}

// generateKML creates a KML document from the waypoints.
func generateKML(waypoints []storage.Waypoint) KML {
	placemarks := make([]Placemark, len(waypoints))
	for i, wp := range waypoints {
		// KML coordinates are in the format: longitude,latitude,altitude
		coords := fmt.Sprintf("%.6f,%.6f,0", wp.Longitude, wp.Latitude)

		// Build description with metadata.
		description := fmt.Sprintf(
			"Sources: %d\nFirst seen: %s\nLast seen: %s",
			wp.SourceCount,
			wp.FirstSeen.Format("2006-01-02 15:04:05 UTC"),
			wp.LastSeen.Format("2006-01-02 15:04:05 UTC"),
		)

		placemarks[i] = Placemark{
			Name:        wp.Name,
			Description: description,
			StyleURL:    "#waypointStyle",
			Point: Point{
				Coordinates: coords,
			},
			ExtendedData: &ExtendedData{
				Data: []Data{
					{Name: "source_count", Value: fmt.Sprintf("%d", wp.SourceCount)},
					{Name: "first_seen", Value: wp.FirstSeen.Format(time.RFC3339)},
					{Name: "last_seen", Value: wp.LastSeen.Format(time.RFC3339)},
				},
			},
		}
	}

	return KML{
		Namespace: "http://www.opengis.net/kml/2.2",
		Document: Document{
			Name:        "ACARS Waypoints",
			Description: fmt.Sprintf("Navigation waypoints extracted from ACARS messages. Generated %s.", time.Now().Format("2006-01-02 15:04:05")),
			Styles: []Style{
				{
					ID: "waypointStyle",
					IconStyle: IconStyle{
						Scale: 0.8,
						Icon: Icon{
							Href: "http://maps.google.com/mapfiles/kml/shapes/triangle.png",
						},
					},
				},
			},
			Placemarks: placemarks,
		},
	}
}

// showWaypointStats displays statistics about the waypoints in the database.
func showWaypointStats(ctx context.Context, pg *storage.PostgresDB) {
	pool := pg.Pool()

	var total int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM waypoints").Scan(&total)

	var avgSources float64
	_ = pool.QueryRow(ctx, "SELECT COALESCE(AVG(source_count), 0) FROM waypoints").Scan(&avgSources)

	var maxSources int
	var maxName string
	_ = pool.QueryRow(ctx, "SELECT name, source_count FROM waypoints ORDER BY source_count DESC LIMIT 1").Scan(&maxName, &maxSources)

	var oldestTime, newestTime *time.Time
	_ = pool.QueryRow(ctx, "SELECT MIN(first_seen), MAX(last_seen) FROM waypoints").Scan(&oldestTime, &newestTime)

	fmt.Println("Waypoint Statistics")
	fmt.Println("───────────────────")
	fmt.Printf("Total waypoints:     %d\n", total)
	fmt.Printf("Average sources:     %.1f\n", avgSources)
	if maxName != "" {
		fmt.Printf("Most observed:       %s (%d sources)\n", maxName, maxSources)
	}
	if oldestTime != nil && newestTime != nil {
		fmt.Printf("Date range:          %s to %s\n", oldestTime.Format("2006-01-02"), newestTime.Format("2006-01-02"))
	}

	// Source count distribution.
	fmt.Println("\nSource Count Distribution:")
	rows, err := pool.Query(ctx, `
		SELECT
			CASE
				WHEN source_count = 1 THEN '1'
				WHEN source_count <= 5 THEN '2-5'
				WHEN source_count <= 10 THEN '6-10'
				WHEN source_count <= 50 THEN '11-50'
				ELSE '50+'
			END as bucket,
			COUNT(*) as cnt
		FROM waypoints
		GROUP BY bucket
		ORDER BY MIN(source_count)
	`)
	if err == nil {
		defer rows.Close()
		fmt.Printf("%-10s %10s\n", "Sources", "Count")
		for rows.Next() {
			var bucket string
			var cnt int
			_ = rows.Scan(&bucket, &cnt)
			fmt.Printf("%-10s %10d\n", bucket, cnt)
		}
	}
}
