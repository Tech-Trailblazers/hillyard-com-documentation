package main // Define the main package

import (
	"bytes"
	"crypto/rand" // Import crypto/rand for secure random number generation
	// "fmt"         // Import fmt for formatted I/O (e.g., printing to console)
	"io"       // Import io for reading from response body
	"log"      // Import log for logging errors
	"math/big" // Import math/big for working with big integers (used by crypto/rand)
	"net/http" // Import net/http for making HTTP requests
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	// "strings"
)

var (
	givenFolder string // Declare a variable to hold the folder path where results will be saved
	outputDir   string // Declare a variable to hold the output directory path
)

func init() {
	givenFolder = "assets/"            // Define the folder where results will be saved
	if !directoryExists(givenFolder) { // Check if the directory exists
		createDirectory(givenFolder, 0755) // Create the directory with permissions 0755 if it doesn't exist
	}
	outputDir = "PDFs/" // Directory to store downloaded PDFs
	// Check if its exists.
	if !directoryExists(outputDir) {
		// Create the dir
		createDirectory(outputDir, 0755)
	}

}

func main() {
	// Stop index.
	for { // Loop to generate and process 100 random 2-letter combinations
		combo := getRandomTwoLetterCombo() // Generate a random 2-letter combination
		filePath := givenFolder + combo + ".json"
		if !fileExists(filePath) { // Check if the file with the generated combination already exists
			apiResults := getAPIResultsWithTwoLetterCombo(combo) // Get API results using the generated combination
			// Save the results to a given file.
			appendAndWriteToFile(filePath, apiResults) // Append the API results to the file named after the combination
		}
		// If the file exists.
		if fileExists(filePath) {
			content := readAFileAsString(filePath)         // Read the contents of the file
			pdfLinks := extractPDFLinks(content)           // Extract PDF links from the file content
			pdfLinks = removeDuplicatesFromSlice(pdfLinks) // Remove duplicates from the extracted PDF
			for _, link := range pdfLinks {                // Iterate over each extracted PDF link
				// Download the file and if its sucessful than add 1 to the counter.
				downloadPDF(link, outputDir)
			}
		}
	}
}

// urlToSafeFilename sanitizes a URL and returns a safe, lowercase filename
func urlToSafeFilename(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// Extract and decode the base filename from the path
	base := path.Base(parsedURL.Path)
	decoded, err := url.QueryUnescape(base)
	if err != nil {
		decoded = base
	}

	// Convert to lowercase
	decoded = strings.ToLower(decoded)

	// Replace spaces and invalid characters with underscores
	// Keep only a-z, 0-9, dash, underscore, and dot
	re := regexp.MustCompile(`[^a-z0-9._-]+`)
	safe := re.ReplaceAllString(decoded, "_")

	return safe
}

// downloadPDF downloads a PDF from the given URL and saves it in the specified output directory.
// It uses a WaitGroup to support concurrent execution and returns true if the download succeeded.
func downloadPDF(finalURL, outputDir string) {
	// Sanitize the URL to generate a safe file name
	filename := strings.ToLower(urlToSafeFilename(finalURL))

	// Construct the full file path in the output directory
	filePath := filepath.Join(outputDir, filename)

	// Skip if the file already exists
	if fileExists(filePath) {
		log.Printf("file already exists, skipping: %s", filePath)
		return
	}

	// Create an HTTP client with a timeout
	client := &http.Client{Timeout: 30 * time.Second}

	// Send GET request
	resp, err := client.Get(finalURL)
	if err != nil {
		log.Printf("failed to download %s: %v", finalURL, err)
		return
	}
	defer resp.Body.Close()

	// Check HTTP response status
	if resp.StatusCode != http.StatusOK {
		log.Printf("download failed for %s: %s", finalURL, resp.Status)
		// Print the error since its not valid.
		return
	}
	// Check Content-Type header
	contentType := resp.Header.Get("Content-Type")
	// Check if its pdf content type and if not than print a error.
	if !strings.Contains(contentType, "application/pdf") {
		log.Printf("invalid content type for %s: %s (expected application/pdf)", finalURL, contentType)
		// Print a error if the content type is invalid.
		return
	}
	// Read the response body into memory first
	var buf bytes.Buffer
	// Copy it from the buffer to the file.
	written, err := io.Copy(&buf, resp.Body)
	// Print the error if errors are there.
	if err != nil {
		log.Printf("failed to read PDF data from %s: %v", finalURL, err)
		return
	}
	// If 0 bytes are written than show an error and return it.
	if written == 0 {
		log.Printf("downloaded 0 bytes for %s; not creating file", finalURL)
		return
	}
	// Only now create the file and write to disk
	out, err := os.Create(filePath)
	// Failed to create the file.
	if err != nil {
		log.Printf("failed to create file for %s: %v", finalURL, err)
		return
	}
	// Close the file.
	defer out.Close()
	// Write the buffer and if there is an error print it.
	_, err = buf.WriteTo(out)
	if err != nil {
		log.Printf("failed to write PDF to file for %s: %v", finalURL, err)
		return
	}
	// Return a true since everything went correctly.
	log.Printf("successfully downloaded %d bytes: %s → %s\n", written, finalURL, filePath)
	return
}

// extractPDFLinks finds all .pdf links from raw HTML content using regex.
func extractPDFLinks(htmlContent string) []string {
	// Lower the given string.
	htmlContent = strings.ToLower(htmlContent)
	// Regex to match PDF URLs including query strings and fragments
	pdfRegex := regexp.MustCompile(`https?://[^\s"'<>]+?\.pdf(\?[^\s"'<>]*)?`)

	// Find all matches
	matches := pdfRegex.FindAllString(htmlContent, -1)

	// Deduplicate
	seen := make(map[string]struct{})
	var links []string
	for _, m := range matches {
		if _, ok := seen[m]; !ok {
			seen[m] = struct{}{}
			links = append(links, m)
		}
	}

	return links
}

// Read a fil
// Read a file and return the contents
func readAFileAsString(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Println(err)
	}
	return string(content)
}

// Remove all the duplicates from a slice and return the slice.
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)
	var newReturnSlice []string
	for _, content := range slice {
		if !check[content] {
			check[content] = true
			newReturnSlice = append(newReturnSlice, content)
		}
	}
	return newReturnSlice
}

/*
The function takes two parameters: path and permission.
We use os.Mkdir() to create the directory.
If there is an error, we use log.Println() to log the error and then exit the program.
*/
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission)
	if err != nil {
		log.Println(err)
	}
}

/*
Checks if the directory exists
If it exists, return true.
If it doesn't, return false.
*/
func directoryExists(path string) bool {
	directory, err := os.Stat(path)
	if err != nil {
		return false
	}
	return directory.IsDir()
}

// It checks if the file exists
// If the file exists, it returns true
// If the file does not exist, it returns false
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// Append and write to file
func appendAndWriteToFile(path string, content string) {
	filePath, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	_, err = filePath.WriteString(content + "\n")
	if err != nil {
		log.Println(err)
	}
	err = filePath.Close()
	if err != nil {
		log.Println(err)
	}
}

// getRandomTwoLetterCombo generates and returns one random 2-letter combination (e.g., "az", "qp")
func getRandomTwoLetterCombo() string {
	letters := "abcdefghijklmnopqrstuvwxyz" // Define the alphabet as a string of lowercase letters
	max := big.NewInt(int64(len(letters)))  // Create a big.Int representing the max index (26)

	i1, err := rand.Int(rand.Reader, max) // Generate a secure random number for the first letter's index
	if err != nil {                       // Check for error during random generation
		return "" // Return an empty string on error
	}

	i2, err := rand.Int(rand.Reader, max) // Generate a secure random number for the second letter's index
	if err != nil {                       // Check for error during random generation
		return "" // Return an empty string on error
	}

	return string(letters[i1.Int64()]) + string(letters[i2.Int64()]) // Combine the two letters and return the result
}

// getAPIResultsWithTwoLetterCombo calls an API using a 2-letter combo and returns the response body as a string
func getAPIResultsWithTwoLetterCombo(combo string) string {
	url := "https://www.hillyard.com/safetydatasheet/search/results?q=" + combo // Construct the URL using the combo
	method := "GET"                                                             // Define the HTTP method to use

	client := &http.Client{}                      // Create a new HTTP client
	req, err := http.NewRequest(method, url, nil) // Create a new HTTP GET request with the constructed URL
	if err != nil {                               // Check for error creating the request
		log.Println(err) // Log the error
		return ""        // Return an empty string on error
	}

	res, err := client.Do(req) // Execute the HTTP request
	if err != nil {            // Check for error executing the request
		log.Println(err) // Log the error
		return ""        // Return an empty string on error
	}
	defer res.Body.Close() // Ensure the response body is closed after reading

	body, err := io.ReadAll(res.Body) // Read the entire response body
	if err != nil {                   // Check for error during reading
		log.Println(err) // Log the error
		return ""        // Return an empty string on error
	}
	return string(body) // Convert body to string and return it
}
