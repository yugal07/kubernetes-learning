package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// This is the GVR (Group, Version, Resource) — it tells the Kubernetes client
// which "table" to query. Think of it as: database.schema.table
var databaseBackupGVR = schema.GroupVersionResource{
	Group:    "learn.example.com", // The group we defined in the CRD
	Version:  "v1",                // The version we defined
	Resource: "databasebackups",   // The plural name from the CRD
}

func main() {
	// -----------------------------------------------
	// Step 1: Connect to the cluster
	// -----------------------------------------------
	// This is like mongoose.connect() — but for Kubernetes instead of MongoDB.
	// It reads ~/.kube/config (which KinD set up for us).
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Failed to load kubeconfig: %v", err)
	}

	// dynamic.NewForConfig gives us a client that can work with ANY resource type,
	// including our custom DatabaseBackup. We don't need generated Go types.
	// Think of it like using MongoDB's raw collection.find() instead of Mongoose models.
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	fmt.Println("Controller started. Watching for DatabaseBackup resources...")

	// -----------------------------------------------
	// Step 2: The Reconciliation Loop
	// -----------------------------------------------
	// This is the core pattern. Every Kubernetes controller does this.
	// Crossplane runs this loop for S3Buckets, RDS instances, etc.
	for {
		reconcile(client)
		time.Sleep(5 * time.Second) // Poll every 5 seconds
	}
}

func reconcile(client dynamic.Interface) {
	ctx := context.Background()

	// LIST all DatabaseBackup resources — like: SELECT * FROM databasebackups
	backups, err := client.Resource(databaseBackupGVR).Namespace("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Error listing backups: %v", err)
		return
	}

	if len(backups.Items) == 0 {
		return
	}

	for _, backup := range backups.Items {
		name := backup.GetName()

		// Read the spec — this is what the user wants
		spec, found, _ := unstructured.NestedMap(backup.Object, "spec")
		if !found {
			continue
		}

		dbName, _, _ := unstructured.NestedString(spec, "databaseName")
		schedule, _, _ := unstructured.NestedString(spec, "schedule")
		retentionDays, _, _ := unstructured.NestedFieldNoCopy(spec, "retentionDays")

		// Read the current status — this is the actual state
		phase, _, _ := unstructured.NestedString(backup.Object, "status", "phase")

		// -----------------------------------------------
		// Step 3: Reconcile — compare desired vs actual
		// -----------------------------------------------
		// If the phase is already "Completed", skip it.
		// A real controller would check if a new backup is due based on the schedule.
		if phase == "Completed" {
			continue
		}

		fmt.Printf("\n--- Reconciling DatabaseBackup: %s ---\n", name)
		fmt.Printf("  Database:      %s\n", dbName)
		fmt.Printf("  Schedule:      %s\n", schedule)
		fmt.Printf("  RetentionDays: %v\n", retentionDays)
		fmt.Printf("  Current Phase: %s\n", phase)

		// -----------------------------------------------
		// Step 4: Take action
		// -----------------------------------------------
		// In a real controller, this is where you'd:
		//   - Crossplane: call AWS API to create an S3 bucket
		//   - A backup tool: trigger a database dump
		// We'll simulate it with a log message.
		fmt.Printf("  ACTION: Simulating backup of database '%s'...\n", dbName)
		time.Sleep(1 * time.Second) // Simulate work
		fmt.Printf("  ACTION: Backup complete!\n")

		// -----------------------------------------------
		// Step 5: Update status — report actual state
		// -----------------------------------------------
		// This is like UPDATE databasebackups SET status = ... WHERE name = ...
		// After this, kubectl get dbb will show the Phase column as "Completed"
		backup.Object["status"] = map[string]interface{}{
			"phase":          "Completed",
			"lastBackupTime": time.Now().UTC().Format(time.RFC3339),
		}

		_, err := client.Resource(databaseBackupGVR).Namespace("default").UpdateStatus(
			ctx, &backup, metav1.UpdateOptions{},
		)
		if err != nil {
			log.Printf("  ERROR updating status: %v", err)
		} else {
			fmt.Printf("  STATUS: Updated phase to 'Completed'\n")
		}
	}
}

// prettyPrint is a helper to debug resources
func prettyPrint(obj interface{}) {
	b, _ := json.MarshalIndent(obj, "", "  ")
	fmt.Println(string(b))
}
