package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	concurrency := 8
	timeout := 8
	retries := 3
	retrySleep := 1

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			Dial:                (&net.Dialer{Timeout: time.Duration(timeout) * time.Second}).Dial,
			TLSHandshakeTimeout: time.Duration(timeout) * time.Second,
		},
	}

	work := make(chan string)
	wg := &sync.WaitGroup{}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range work {
				checkURL(url, client, retries, retrySleep)
			}
		}()
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			work <- line
		}
	}
	close(work)
	wg.Wait()
}

func checkURL(urlStr string, client *http.Client, retries, retrySleep int) {
	payload := "kzxss"

	// Parse URL
	parsed, err := url.Parse(urlStr)
	if err != nil {
		fmt.Println("[!] Invalid URL:", urlStr, err)
		return
	}

	// Get the original query parameters just once.
	originalQuery := parsed.Query()
	// Also save the original raw query string to easily restore it later.
	originalRawQuery := parsed.RawQuery 

	// Loop over the *keys* of the original query.
	for param := range originalQuery {
		// --- Test GET ---
		// Create a fresh copy of the original query for this iteration.
		modifiedQuery := originalQuery
		modifiedQuery.Set(param, payload)
		
		// Restore the original query before creating the test URL for GET
		parsed.RawQuery = originalRawQuery
		
		// Create a temporary URL object for this specific test
		var tempURL url.URL = *parsed 
		tempURL.RawQuery = modifiedQuery.Encode()
		testURL := tempURL.String()

		for attempt := 0; attempt <= retries; attempt++ {
			req, err := http.NewRequest("GET", testURL, nil)
			if err != nil {
				fmt.Println("[!] GET request creation failed:", testURL, err)
				break
			}
			req.Header.Set("Connection", "close")

			resp, err := client.Do(req)
			if err != nil {
				if attempt < retries {
					time.Sleep(time.Duration(retrySleep) * time.Second)
					continue
				}
				fmt.Println("[!] GET request failed:", testURL, err)
				break
			}

			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1_000_000))
			resp.Body.Close()

			if strings.Contains(string(body), payload) {
				fmt.Printf("[REFLECTION:GET] %s (param: %s)\n", testURL, param)
			}
			break
		}

		// --- Test POST ---
		// Your POST logic was already correct because it built `postData` from scratch
		// using `originalQuery`, so it didn't suffer from the mutation bug.
		// No changes are needed here.
		postData := url.Values{}
		for k := range originalQuery {
			if k == param {
				postData.Set(k, payload)
			} else {
				postData.Set(k, originalQuery.Get(k))
			}
		}

		// (The rest of your POST logic remains the same)
		for attempt := 0; attempt <= retries; attempt++ {
			// Create the base URL for the POST request (without query params)
			postURL := parsed.Scheme + "://" + parsed.Host + parsed.Path
			req, err := http.NewRequest("POST", postURL, strings.NewReader(postData.Encode()))
			if err != nil {
				fmt.Println("[!] POST request creation failed:", urlStr, err)
				break
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Connection", "close")

			resp, err := client.Do(req)
			if err != nil {
				if attempt < retries {
					time.Sleep(time.Duration(retrySleep) * time.Second)
					continue
				}
				fmt.Println("[!] POST request failed:", postURL, err)
				break
			}

			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1_000_000))
			resp.Body.Close()

			if strings.Contains(string(body), payload) {
				// We print the original URL for clarity in POST reflections
				fmt.Printf("[REFLECTION:POST] %s (param: %s)\n", urlStr, param)
			}
			break
		}
	}
}