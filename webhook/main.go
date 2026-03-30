package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	// Our webhook is just an HTTP server — like an Express app.
	// Kubernetes will POST admission review requests to /validate.
	http.HandleFunc("/validate", handleValidate)

	fmt.Println("Webhook server starting on :8443...")

	// Kubernetes REQUIRES webhooks to use HTTPS.
	// We'll generate self-signed certs in the challenge steps.
	err := http.ListenAndServeTLS(":8443", "tls.crt", "tls.key", nil)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func handleValidate(w http.ResponseWriter, r *http.Request) {
	// -----------------------------------------------
	// Step 1: Read the AdmissionReview request
	// -----------------------------------------------
	// Kubernetes sends us a JSON body called an AdmissionReview.
	// It contains the full resource the user is trying to create.
	// Think of it as: req.body in Express, but with a specific structure.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read body", http.StatusBadRequest)
		return
	}

	var admissionReview admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &admissionReview); err != nil {
		http.Error(w, "could not unmarshal request", http.StatusBadRequest)
		return
	}

	// -----------------------------------------------
	// Step 2: Extract the Pod from the request
	// -----------------------------------------------
	// The actual Pod YAML the user submitted is inside request.object.raw
	var pod corev1.Pod
	if err := json.Unmarshal(admissionReview.Request.Object.Raw, &pod); err != nil {
		http.Error(w, "could not unmarshal pod", http.StatusBadRequest)
		return
	}

	fmt.Printf("Received admission request for Pod: %s/%s\n", pod.Namespace, pod.Name)

	// -----------------------------------------------
	// Step 3: Apply our policy — THE VALIDATION LOGIC
	// -----------------------------------------------
	// This is what Kyverno does! Kyverno reads your ClusterPolicy CRs
	// and generates this kind of check dynamically.
	// We're hardcoding it: "every Pod must have a 'team' label."
	allowed := true
	reason := ""

	if pod.Labels == nil || pod.Labels["team"] == "" {
		allowed = false
		reason = "DENIED: Pod must have a 'team' label. Add metadata.labels.team to your Pod spec."
		fmt.Printf("  REJECTED: Pod '%s' has no 'team' label\n", pod.Name)
	} else {
		fmt.Printf("  ALLOWED: Pod '%s' has team label: %s\n", pod.Name, pod.Labels["team"])
	}

	// -----------------------------------------------
	// Step 4: Send the response back to Kubernetes
	// -----------------------------------------------
	// We respond with an AdmissionReview that says "allowed: true/false"
	// If denied, the user sees our reason in their kubectl output.
	response := admissionv1.AdmissionReview{
		// Must echo back the same API version and kind
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: &admissionv1.AdmissionResponse{
			UID:     admissionReview.Request.UID, // Must echo back the UID
			Allowed: allowed,
		},
	}

	if !allowed {
		response.Response.Result = &metav1.Status{
			Message: reason,
		}
	}

	respBytes, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "could not marshal response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(respBytes)
}
