package main

import (
	"encoding/json"
	"fmt"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

const (
	tlsKeyName  = "tls.key"
	tlsCertName = "tls.crt"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", validate)
	if certDir := os.Getenv("CERT_DIR"); certDir != "" {
		certFile := filepath.Join(certDir, tlsCertName)
		keyFile := filepath.Join(certDir, tlsKeyName)
		log.Fatal(http.ListenAndServeTLS(":8000", certFile, keyFile, mux))
	} else {
		log.Fatal(http.ListenAndServe(":8000", mux))
	}
}

func validate(w http.ResponseWriter, r *http.Request) {
	log.Println("Received request")
	var (
		reviewReq, reviewResp admissionv1.AdmissionReview
		pd                    corev1.Pod
	)

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&reviewReq); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Failed to decode request: %v", err)
		return
	}

	if err := json.Unmarshal(reviewReq.Request.Object.Raw, &pd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Failed to unmarshal pod object: %v", err)
		return
	}
	log.Printf("Validating pod: %s", pd.Name)

	reviewResp.TypeMeta = reviewReq.TypeMeta
	reviewResp.Response = &admissionv1.AdmissionResponse{
		UID:     reviewReq.Request.UID,
		Allowed: true,
		Result:  nil,
	}

	for _, ctr := range pd.Spec.Containers {
		if len(ctr.Env) > 0 {
			reviewResp.Response.Allowed = false
			reviewResp.Response.Result = &metav1.Status{
				Status:  "Failure",
				Message: fmt.Sprintf("%s is using env vars", ctr.Name),
				Reason:  metav1.StatusReason(fmt.Sprintf("%s is using env vars", ctr.Name)),
				Code:    402,
			}
			break
		}
	}

	js, err := json.Marshal(reviewResp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Failed to marshal response: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}
