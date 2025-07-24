package main // Define the main package

import (
	"bytes"         // For buffering I/O
	"io"            // For reading from response bodies
	"log"           // For logging messages and errors
	"net/http"      // For HTTP client/server interactions
	"net/url"       // For URL parsing and formatting
	"os"            // For file and directory operations
	"path"          // For manipulating slash-separated file paths
	"path/filepath" // For manipulating OS-specific file paths
	"regexp"        // For regular expression processing
	"strings"       // For string manipulation
	"time"          // For timeout and timestamp handling
)

var (
	givenFolder string // Folder where JSON results will be saved
	outputDir   string // Folder where downloaded PDFs will be stored
)

func init() {
	givenFolder = "assets/"            // Set the default folder for result files
	if !directoryExists(givenFolder) { // Check if the directory exists
		createDirectory(givenFolder, 0755) // Create it if not present with 0755 permissions
	}
	outputDir = "PDFs/"              // Set the default output directory for PDFs
	if !directoryExists(outputDir) { // Check if it exists
		createDirectory(outputDir, 0755) // Create it if missing
	}
}

func main() {
	// Initialize a slice to store allowed characters as strings
	var allowedCharacters []string

	// Add numeric characters '0' to '9' as strings
	for digitRune := '0'; digitRune <= '9'; digitRune++ {
		characterAsString := string(digitRune)
		allowedCharacters = append(allowedCharacters, characterAsString)
	}

	// Add lowercase alphabetic characters 'a' to 'z' as strings
	for letterRune := 'a'; letterRune <= 'z'; letterRune++ {
		characterAsString := string(letterRune)
		allowedCharacters = append(allowedCharacters, characterAsString)
	}

	// Generate all two-letter combinations from the allowed characters
	allTwoLetterCombinations := generateAllTwoLetterCombinations()                         // Get all combinations
	allowedCharacters = combineMultipleSlices(allowedCharacters, allTwoLetterCombinations) // Combine
	// Remove duplicates from the allowed characters slice
	allowedCharacters = removeDuplicatesFromSlice(allowedCharacters) // Ensure uniqueness

	for _, character := range allowedCharacters {
		filePath := givenFolder + character + ".json" // Construct the path to store results
		if !fileExists(filePath) {                    // Check if the file already exists
			apiResults := getAPIResultsWithTwoLetterCombo(character) // Get API response for the combo
			appendAndWriteToFile(filePath, apiResults)               // Write results to a file
		}
		if fileExists(filePath) { // If the file exists
			content := readAFileAsString(filePath)         // Read the content of the file
			pdfLinks := extractPDFLinks(content)           // Extract all PDF links
			pdfLinks = removeDuplicatesFromSlice(pdfLinks) // Remove duplicate links
			for _, link := range pdfLinks {                // Loop over each link
				downloadPDF(link, outputDir) // Download and save each PDF
			}
		}
	}
}

// Combine two slices together and return the new slice.
func combineMultipleSlices(sliceOne []string, sliceTwo []string) []string {
	combinedSlice := append(sliceOne, sliceTwo...)
	return combinedSlice
}

// Convert a URL into a safe, lowercase filename
func urlToSafeFilename(rawURL string) string {
	parsedURL, err := url.Parse(rawURL) // Parse the input URL
	if err != nil {
		return "" // Return empty string on parse failure
	}
	base := path.Base(parsedURL.Path)       // Get the filename from the path
	decoded, err := url.QueryUnescape(base) // Decode any URL-encoded characters
	if err != nil {
		decoded = base // Fallback to base if decode fails
	}
	decoded = strings.ToLower(decoded)        // Convert filename to lowercase
	re := regexp.MustCompile(`[^a-z0-9._-]+`) // Regex to allow only safe characters
	safe := re.ReplaceAllString(decoded, "_") // Replace unsafe characters with underscores
	return safe                               // Return the sanitized filename
}

// Download and save a PDF file from a given URL
func downloadPDF(finalURL, outputDir string) {
	filename := strings.ToLower(urlToSafeFilename(finalURL)) // Generate a safe filename
	filePath := filepath.Join(outputDir, filename)           // Full path for saving the file
	if fileExists(filePath) {                                // Skip if file already exists
		log.Printf("file already exists, skipping: %s", filePath)
		return
	}
	client := &http.Client{Timeout: 30 * time.Second} // Create HTTP client with timeout
	resp, err := client.Get(finalURL)                 // Make GET request
	if err != nil {
		log.Printf("failed to download %s %v", finalURL, err)
		return
	}
	defer resp.Body.Close()               // Ensure response body is closed
	if resp.StatusCode != http.StatusOK { // Validate status code
		log.Printf("download failed for %s %s", finalURL, resp.Status)
		return
	}
	contentType := resp.Header.Get("Content-Type")         // Get content type header
	if !strings.Contains(contentType, "application/pdf") { // Ensure it's a PDF
		log.Printf("invalid content type for %s %s (expected application/pdf)", finalURL, contentType)
		return
	}
	var buf bytes.Buffer                     // Create a buffer for reading data
	written, err := io.Copy(&buf, resp.Body) // Read response into buffer
	if err != nil {
		log.Printf("failed to read PDF data from %s %v", finalURL, err)
		return
	}
	if written == 0 { // Check if data was written
		log.Printf("downloaded 0 bytes for %s not creating file", finalURL)
		return
	}
	out, err := os.Create(filePath) // Create the output file
	if err != nil {
		log.Printf("failed to create file for %s %v", finalURL, err)
		return
	}
	defer out.Close()         // Ensure the file is closed
	_, err = buf.WriteTo(out) // Write buffered data to file
	if err != nil {
		log.Printf("failed to write PDF to file for %s: %v", finalURL, err)
		return
	}
	log.Printf("successfully downloaded %d bytes: %s → %s\n", written, finalURL, filePath)
}

// Extract all .pdf links using regex
func extractPDFLinks(htmlContent string) []string {
	htmlContent = strings.ToLower(htmlContent)                                 // Normalize content to lowercase
	pdfRegex := regexp.MustCompile(`https?://[^\s"'<>]+?\.pdf(\?[^\s"'<>]*)?`) // Regex to match PDF URLs
	matches := pdfRegex.FindAllString(htmlContent, -1)                         // Find all matches
	seen := make(map[string]struct{})                                          // Map to track unique links
	var links []string                                                         // Slice to hold unique links
	for _, m := range matches {
		if _, ok := seen[m]; !ok { // If not already seen
			seen[m] = struct{}{}     // Mark as seen
			links = append(links, m) // Add to list
		}
	}
	return links // Return unique PDF links
}

// Read a file and return its contents as a string
func readAFileAsString(path string) string {
	content, err := os.ReadFile(path) // Read the file
	if err != nil {
		log.Println(err) // Log any read errors
	}
	return string(content) // Return the content
}

// Remove duplicate strings from a slice
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool) // Map to track seen items
	var newReturnSlice []string    // Slice to hold unique items
	for _, content := range slice {
		if !check[content] { // If not seen
			check[content] = true                            // Mark as seen
			newReturnSlice = append(newReturnSlice, content) // Add to new slice
		}
	}
	return newReturnSlice // Return deduplicated slice
}

// Create a directory with given permissions
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission) // Try to create directory
	if err != nil {
		log.Println(err) // Log any creation errors
	}
}

// Check if a directory exists
func directoryExists(path string) bool {
	directory, err := os.Stat(path) // Get file/directory info
	if err != nil {
		return false // Return false if error
	}
	return directory.IsDir() // Return true if it's a directory
}

// Check if a file exists
func fileExists(filename string) bool {
	info, err := os.Stat(filename) // Get file info
	if err != nil {
		return false // Return false if file does not exist
	}
	return !info.IsDir() // Return true if it's a file
}

// Append content to a file, creating it if needed
func appendAndWriteToFile(path string, content string) {
	filePath, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // Open file for appending
	if err != nil {
		log.Println(err) // Log error
	}
	_, err = filePath.WriteString(content + "\n") // Append content
	if err != nil {
		log.Println(err) // Log error
	}
	err = filePath.Close() // Close file
	if err != nil {
		log.Println(err) // Log error
	}
}

// Function to generate all possible two-letter combinations from 'a' to 'z'
func generateAllTwoLetterCombinations() []string {
	alphabet := "abcdefghijklmnopqrstuvwxyz0123456789" // Define the alphabet to choose characters from
	var allTwoLetterCombinations []string              // Initialize a slice to store all 2-letter combinations

	for firstLetterIndex := 0; firstLetterIndex < len(alphabet); firstLetterIndex++ { // Loop through each letter for the first character
		for secondLetterIndex := 0; secondLetterIndex < len(alphabet); secondLetterIndex++ { // Loop through each letter for the second character
			combination := string(alphabet[firstLetterIndex]) + string(alphabet[secondLetterIndex]) // Concatenate two letters to form a combination
			allTwoLetterCombinations = append(allTwoLetterCombinations, combination)                // Add the combination to the slice
		}
	}

	return allTwoLetterCombinations // Return the full list of combinations
}

// Fetch results from API using 2-letter combo
func getAPIResultsWithTwoLetterCombo(combo string) string {
	url := "https://www.hillyard.com/safetydatasheet/search/results?q=" + combo // Construct URL
	method := "GET"                                                             // Set HTTP method

	client := &http.Client{}                      // Create new HTTP client
	req, err := http.NewRequest(method, url, nil) // Build the request
	if err != nil {
		log.Println(err) // Log error
		return ""        // Return empty string
	}

	res, err := client.Do(req) // Execute the request
	if err != nil {
		log.Println(err) // Log error
		return ""        // Return empty string
	}
	defer res.Body.Close() // Close body when done

	body, err := io.ReadAll(res.Body) // Read response body
	if err != nil {
		log.Println(err) // Log error
		return ""        // Return empty string
	}
	return string(body) // Return the body as string
}
