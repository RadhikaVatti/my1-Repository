package main

import (
	"context"
	"fmt"
	"log"
	"math"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Haversine distance in meters
func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // Radius of Earth in meters
	lat1Rad := lat1 * (math.Pi / 180)
	lon1Rad := lon1 * (math.Pi / 180)
	lat2Rad := lat2 * (math.Pi / 180)
	lon2Rad := lon2 * (math.Pi / 180)

	deltaLat := lat2Rad - lat1Rad
	deltaLon := lon2Rad - lon1Rad

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

func objectIDToString(id primitive.ObjectID) string {
	return id.Hex()
}

func main() {

	imei := int64(350317173261953) // enter imei

	// MongoDB connection
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017") // Update with your MongoDB URI
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.TODO())

	db := client.Database("inout")
	telematicsCollection := db.Collection("teledata")
	zonesCollection := db.Collection("zone")
	vehiclesCollection := db.Collection("Vehicledata")

	// Fetch telematics data
	telematicsCursor, err := telematicsCollection.Find(context.TODO(), bson.D{})
	if err != nil {
		log.Fatal(err)
	}
	defer telematicsCursor.Close(context.TODO())

	// Fetch zone data
	zonesCursor, err := zonesCollection.Find(context.TODO(), bson.D{})
	if err != nil {
		log.Fatal(err)
	}
	defer zonesCursor.Close(context.TODO())

	// Fetch vehicle data
	vehiclesCursor, err := vehiclesCollection.Find(context.TODO(), bson.D{})
	if err != nil {
		log.Fatal(err)
	}
	defer vehiclesCursor.Close(context.TODO())

	// Map
	zones := make(map[string]bson.M)
	for zonesCursor.Next(context.TODO()) {
		var zone bson.M
		if err = zonesCursor.Decode(&zone); err != nil {
			log.Fatal(err)
		}
		zoneID, ok := zone["_id"].(primitive.ObjectID)
		if !ok {
			log.Fatalf("Unexpected type for zone _id: %T", zone["_id"])
		}
		zones[objectIDToString(zoneID)] = zone
	}

	// Find the vehicle data for the given IMEI
	vehicle := bson.M{}
	filter := bson.M{"imei": imei}
	err = vehiclesCollection.FindOne(context.TODO(), filter).Decode(&vehicle)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Println("Vehicle not found:", imei)
		} else {
			log.Println("Error finding vehicle:", err)
		}
		return
	}

	zoneID, ok := vehicle["zoneID"].(primitive.ObjectID)
	if !ok {
		log.Println("Zone ID not found or not in expected format")
		return
	}

	zone, found := zones[objectIDToString(zoneID)]
	if !found {
		log.Println("Zone not found:", zoneID)
		return
	}

	zoneLat, ok := zone["latitude"].(float64)
	if !ok {
		log.Fatalf("Error parsing zone latitude: %v", err)
	}
	zoneLon, ok := zone["longitude"].(float64)
	if !ok {
		log.Fatalf("Error parsing zone longitude: %v", err)
	}
	radius, ok := zone["radius"].(int32)
	if !ok {
		log.Fatalf("Error parsing zone radius: %v", err)
	}

	// Counters for inbound and outbound
	inboundCount := 0
	outboundCount := 0

	// Iterate over telematics data
	for telematicsCursor.Next(context.TODO()) {
		var telematics bson.M
		if err = telematicsCursor.Decode(&telematics); err != nil {
			log.Fatal(err)
		}

		// Check if IMEI matches
		if telematics["imei"].(int64) != imei {
			continue
		}

		// Extract GPS latitude and longitude
		gps := telematics["gps"].(bson.M)
		gpsLat, ok := gps["latitude"].(float64)
		if !ok {
			log.Fatalf("Error parsing GPS latitude: %v", err)
		}
		gpsLon, ok := gps["longitude"].(float64)
		if !ok {
			log.Fatalf("Error parsing GPS longitude: %v", err)
		}

		// Calculate distance
		distance := haversineDistance(gpsLat, gpsLon, zoneLat, zoneLon)
		fmt.Printf("GPS Coordinates: (%.6f, %.6f), Zone Coordinates: (%.6f, %.6f)\n", gpsLat, gpsLon, zoneLat, zoneLon)
		fmt.Printf("Distance: %.2f meters\n", distance)

		// Check if the distance is within the zone radius
		if distance < float64(radius) {
			fmt.Printf("Vehicle %s is inbound to zone %s.\n", vehicle["registrationNumber"].(string), zone["name"].(string))
			inboundCount++
		} else {
			fmt.Printf("Vehicle %s is outbound from zone %s.\n", vehicle["registrationNumber"].(string), zone["name"].(string))
			outboundCount++
		}
	}

	// Print the counts
	fmt.Printf("Number of inbound records: %d\n", inboundCount)
	fmt.Printf("Number of outbound records: %d\n", outboundCount)
}
