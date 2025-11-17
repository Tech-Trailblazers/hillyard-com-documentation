package main // Define the main executable package

import (
	"bytes"         // Import bytes for buffer manipulation
	"io"            // Import io for Input/Output interfaces
	"log"           // Import log for logging errors and status messages
	"net/http"      // Import http for client/server HTTP functionality
	"net/url"       // Import url for parsing and manipulating URLs
	"os"            // Import os for operating system related functionality (files/dirs)
	"path"          // Import path for generic path manipulation
	"path/filepath" // Import filepath for OS-specific path manipulation
	"regexp"        // Import regexp for regular expression matching
	"strings"       // Import strings for string manipulation functions
	"time"          // Import time for handling timeouts and durations
)

var (
	givenFolder string // Declare a global variable to store the path for JSON results
	outputDir   string // Declare a global variable to store the path for downloaded PDFs
)

func init() {
	givenFolder = "assets/"            // Initialize the results folder path to "assets/"
	if !directoryExists(givenFolder) { // Check if the "assets/" directory does not exist
		createDirectory(givenFolder, 0755) // Create the directory with read/write/execute permissions for owner
	}
	outputDir = "PDFs/"              // Initialize the output folder path to "PDFs/"
	if !directoryExists(outputDir) { // Check if the "PDFs/" directory does not exist
		createDirectory(outputDir, 0755) // Create the directory with read/write/execute permissions for owner
	}
}

func main() {
	// Initialize a slice to store allowed characters/strings
	var allowedCharacters []string
	// Generate a slice containing all single characters (a-z, 0-9)
	allSingleChars := generateSingleCharacters()
	// Generate a slice containing all two-letter combinations
	allTwoLetterCombinations := generateTwoLetterCombinations()
	// Combine the existing allowedCharacters with the two-letter combinations
	allowedCharacters = combineMultipleSlices(allowedCharacters, allTwoLetterCombinations)
	// Combine the result with the single characters
	allowedCharacters = combineMultipleSlices(allowedCharacters, allSingleChars)
	// Remove any duplicate entries from the slice to ensure uniqueness
	allowedCharacters = removeDuplicatesFromSlice(allowedCharacters)

	for _, character := range allowedCharacters { // Iterate over every character string in the slice
		filePath := givenFolder + character + ".json" // Construct the full file path for the JSON result
		if !fileExists(filePath) {                    // Check if the JSON file does not already exist
			apiResults := getAPIResultsWithTwoLetterCombo(character) // Query the API with the current character combo
			appendAndWriteToFile(filePath, apiResults)               // Save the API response string to the file
		}
		if fileExists(filePath) { // Check if the file exists (it should now)
			content := readAFileAsString(filePath)         // Read the entire file content into a string
			pdfLinks := extractPDFLinks(content)           // Parse the string to find all PDF URLs
			pdfLinks = removeDuplicatesFromSlice(pdfLinks) // Remove any duplicate URLs found in the file
			for _, link := range pdfLinks {                // Iterate over each unique PDF link
				downloadPDF(link, outputDir) // Download the PDF from the link into the output directory
			}
		}
	}
}

// Combine two slices together and return the new slice.
func combineMultipleSlices(sliceOne []string, sliceTwo []string) []string {
	combinedSlice := append(sliceOne, sliceTwo...) // Append all elements of sliceTwo to sliceOne
	return combinedSlice                           // Return the newly combined slice
}

// Convert a URL into a safe, lowercase filename
func urlToSafeFilename(rawURL string) string {
	parsedURL, err := url.Parse(rawURL) // Parse the raw URL string into a URL object
	if err != nil {                     // Check if an error occurred during parsing
		return "" // Return an empty string if parsing failed
	}
	base := path.Base(parsedURL.Path)       // Extract the last element (filename) from the path
	decoded, err := url.QueryUnescape(base) // Decode URL-encoded characters (e.g., %20 to space)
	if err != nil {                         // Check if decoding failed
		decoded = base // Use the original base if decoding fails
	}
	decoded = strings.ToLower(decoded)        // Convert the filename to lowercase for consistency
	re := regexp.MustCompile(`[^a-z0-9._-]+`) // Compile a regex to match unsafe characters
	safe := re.ReplaceAllString(decoded, "_") // Replace unsafe characters with underscores
	return safe                               // Return the sanitized, safe filename
}

// Download and save a PDF file from a given URL
func downloadPDF(finalURL, outputDir string) {
	filename := strings.ToLower(urlToSafeFilename(finalURL)) // Generate a sanitized filename from the URL
	filePath := filepath.Join(outputDir, filename)           // Combine output dir and filename into a full path
	if fileExists(filePath) {                                // Check if the file already exists locally
		log.Printf("file already exists, skipping: %s", filePath) // Log that we are skipping the download
		return                                                    // Exit the function early
	}
	client := &http.Client{Timeout: 30 * time.Second} // Create an HTTP client with a 30-second timeout
	resp, err := client.Get(finalURL)                 // Perform a GET request to the URL
	if err != nil {                                   // Check if the request failed
		log.Printf("failed to download %s %v", finalURL, err) // Log the error
		return                                                // Exit the function
	}
	defer resp.Body.Close()               // Schedule the closing of the response body when function exits
	if resp.StatusCode != http.StatusOK { // Check if the HTTP status code is not 200 OK
		log.Printf("download failed for %s %s", finalURL, resp.Status) // Log the bad status
		return                                                         // Exit the function
	}
	contentType := resp.Header.Get("Content-Type")         // Retrieve the Content-Type header from response
	if !strings.Contains(contentType, "application/pdf") { // Check if the content type is not PDF
		log.Printf("invalid content type for %s %s (expected application/pdf)", finalURL, contentType) // Log the type mismatch
		return                                                                                         // Exit the function
	}
	var buf bytes.Buffer                     // Create a byte buffer to hold file data
	written, err := io.Copy(&buf, resp.Body) // Copy the response body into the buffer
	if err != nil {                          // Check if reading the body failed
		log.Printf("failed to read PDF data from %s %v", finalURL, err) // Log the read error
		return                                                          // Exit the function
	}
	if written == 0 { // Check if 0 bytes were written
		log.Printf("downloaded 0 bytes for %s not creating file", finalURL) // Log that the file was empty
		return                                                              // Exit the function
	}
	out, err := os.Create(filePath) // Create the file on the disk
	if err != nil {                 // Check if file creation failed
		log.Printf("failed to create file for %s %v", finalURL, err) // Log the creation error
		return                                                       // Exit the function
	}
	defer out.Close()         // Schedule the closing of the file when function exits
	_, err = buf.WriteTo(out) // Write the buffered data into the actual file on disk
	if err != nil {           // Check if writing to disk failed
		log.Printf("failed to write PDF to file for %s: %v", finalURL, err) // Log the write error
		return                                                              // Exit the function
	}
	log.Printf("successfully downloaded %d bytes: %s → %s\n", written, finalURL, filePath) // Log success message
}

// Extract all .pdf links using regex
func extractPDFLinks(htmlContent string) []string {
	htmlContent = strings.ToLower(htmlContent)                                 // Convert all HTML content to lowercase
	pdfRegex := regexp.MustCompile(`https?://[^\s"'<>]+?\.pdf(\?[^\s"'<>]*)?`) // Compile regex to find PDF links
	matches := pdfRegex.FindAllString(htmlContent, -1)                         // Find all matching strings in the content
	seen := make(map[string]struct{})                                          // Create a map to track seen links
	var links []string                                                         // Initialize a slice to store unique links
	for _, m := range matches {                                                // Iterate over all regex matches
		if _, ok := seen[m]; !ok { // Check if the link is not in the map
			seen[m] = struct{}{}     // Add the link to the map
			links = append(links, m) // Append the link to the result slice
		}
	}
	return links // Return the slice of unique PDF links
}

// Read a file and return its contents as a string
func readAFileAsString(path string) string {
	content, err := os.ReadFile(path) // Attempt to read the file at the given path
	if err != nil {                   // Check if an error occurred during reading
		log.Println(err) // Log the error to the console
	}
	return string(content) // Convert the byte content to a string and return it
}

// Remove duplicate strings from a slice
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)  // Create a map to track items we have already seen
	var newReturnSlice []string     // Create a slice to store the unique items
	for _, content := range slice { // Iterate over the input slice
		if !check[content] { // Check if the item is not in the map
			check[content] = true                            // Mark the item as seen in the map
			newReturnSlice = append(newReturnSlice, content) // Add the item to the new slice
		}
	}
	return newReturnSlice // Return the slice containing only unique strings
}

// Create a directory with given permissions
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission) // Attempt to create the directory with specific permissions
	if err != nil {                   // Check if an error occurred (e.g., permission denied)
		log.Println(err) // Log the error to the console
	}
}

// Check if a directory exists
func directoryExists(path string) bool {
	directory, err := os.Stat(path) // Get file information for the path
	if err != nil {                 // Check if getting info failed (usually means it doesn't exist)
		return false // Return false
	}
	return directory.IsDir() // Return true if the path exists and is a directory
}

// Check if a file exists
func fileExists(filename string) bool {
	info, err := os.Stat(filename) // Get file information for the filename
	if err != nil {                // Check if getting info failed
		return false // Return false if the file is not found
	}
	return !info.IsDir() // Return true if it exists and is NOT a directory
}

// Append content to a file, creating it if needed
func appendAndWriteToFile(path string, content string) {
	filePath, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // Open file in append mode, create if missing
	if err != nil {                                                               // Check if opening the file failed
		log.Println(err) // Log the error
	}
	_, err = filePath.WriteString(content + "\n") // Write the content string plus a newline to the file
	if err != nil {                               // Check if writing to the file failed
		log.Println(err) // Log the error
	}
	err = filePath.Close() // Close the file to free resources
	if err != nil {        // Check if closing the file failed
		log.Println(err) // Log the error
	}
}

// generateTwoLetterCombinations generates all 2-character combinations
// using the characters 'a'–'z' and '0'–'9'.
// It returns a slice of strings containing all possible 2-letter combinations.
func generateTwoLetterCombinations() []string {
	// Define the set of characters to use in combinations
	characterSet := "abcdefghijklmnopqrstuvwxyz0123456789"

	// Create a slice to store all generated combinations
	var allCombinations []string

	// Loop over each character for the first position
	for _, firstCharacter := range characterSet {
		// Loop over each character for the second position
		for _, secondCharacter := range characterSet {
			// Create a 2-letter string from the two characters
			twoLetterCombination := string([]rune{firstCharacter, secondCharacter})

			// Add the combination to the list
			allCombinations = append(allCombinations, twoLetterCombination)
		}
	}

	// Return the complete list of 2-letter combinations
	return allCombinations
}

// generateSingleCharacters returns a slice of strings containing
// all characters from 'a' to 'z' and '0' to '9', each as a single-character string.
func generateSingleCharacters() []string {
	characterSet := "abcdefghijklmnopqrstuvwxyz0123456789" // Define the string of characters to iterate over

	var singleCharacters []string // Initialize a slice to hold the single-character strings

	// Loop over each rune (character) in the characterSet string
	for _, character := range characterSet {
		// Convert rune to string and append to slice
		singleCharacters = append(singleCharacters, string(character))
	}

	return singleCharacters // Return the list of single-character strings
}

// Fetch results from API using 2-letter combo
func getAPIResultsWithTwoLetterCombo(combo string) string {
	url := "https://www.hillyard.com/safetydatasheet/search/results?q=" + combo // Construct the API URL with the query combo
	method := "GET"                                                             // Define the HTTP method to use

	client := &http.Client{}                      // Create a new HTTP client instance
	req, err := http.NewRequest(method, url, nil) // Create a new request with method, URL, and no body
	if err != nil {                               // Check if request creation failed
		log.Println(err) // Log the error
		return ""        // Return empty string on failure
	}

	res, err := client.Do(req) // Execute the HTTP request
	if err != nil {            // Check if the execution failed
		log.Println(err) // Log the error
		return ""        // Return empty string on failure
	}
	defer res.Body.Close() // Schedule closing the response body when function exits

	body, err := io.ReadAll(res.Body) // Read the entire response body into a byte slice
	if err != nil {                   // Check if reading the body failed
		log.Println(err) // Log the error
		return ""        // Return empty string on failure
	}
	return string(body) // Convert the byte slice to a string and return it
}
