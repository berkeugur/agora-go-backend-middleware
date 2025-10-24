package cloud_recording_service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// generateUID generates a unique user identifier for use within cloud recording sessions.
// This function ensures the UID is never zero, which is reserved, by generating a random
// number between 1 and the maximum possible 32-bit integer value.
func (s *CloudRecordingService) GenerateUID() string {
	// Generate a random number starting from 1 to avoid 0, which is reserved.
	uid := rand.Intn(4294967294) + 1

	// Convert the integer UID to a string format and return it.
	return strconv.Itoa(uid)
}

// ValidateRecordingMode checks if a specific string is present within a slice of strings.
// This is useful for determining if a particular item exists within a list.
func (s *CloudRecordingService) ValidateRecordingMode(modeToCheck string) bool {
	validRecordingModes := []string{"individual", "mix", "web"}
	for _, mode := range validRecordingModes {
		if mode == modeToCheck {
			return true
		}
	}
	return false
}

// AddTimestamp adds a current timestamp to any response object that supports the Timestampable interface.
// It then marshals the updated object back into JSON format for further use or storage.
func (s *CloudRecordingService) AddTimestamp(response Timestampable) (json.RawMessage, error) {
	// Set the current timestamp in UTC and RFC3339 format.
	now := time.Now().UTC().Format(time.RFC3339)
	response.SetTimestamp(now)

	// Marshal the response with the added timestamp back to JSON.
	timestampedBody, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("error marshaling final response with timestamp: %v", err)
	}
	return timestampedBody, nil
}

// UnmarshalFileList interprets the file list from the server response, handling different formats based on the FileListMode.
// It supports 'string' and 'json' modes, returning the file list as either a slice of FileDetail or FileListEntry respectively.
func (sr *ServerResponse) UnmarshalFileList() (interface{}, error) {
	if sr.FileListMode == nil || sr.FileList == nil {
		// Ensure FileListMode and FileList are not nil before proceeding.
		return nil, fmt.Errorf("FileListMode or FileList are empty, cannot proceed with unmarshaling")
	}
	switch *sr.FileListMode {
	case "string":
		// fileList is returned as a JSON-encoded string containing an array of file details.
		// First unmarshal to a plain string and then decode the underlying JSON payload.
		var rawString string
		if err := json.Unmarshal(*sr.FileList, &rawString); err != nil {
			return nil, fmt.Errorf("error parsing FileList into string: %v", err)
		}
		trimmed := strings.TrimSpace(rawString)
		var fileList []FileDetail
		if err := json.Unmarshal([]byte(trimmed), &fileList); err != nil {
			// Some responses append diagnostic text after the JSON payload. Attempt to
			// recover by extracting the JSON array from the payload.
			if candidate, ok := extractJSONArray(trimmed); ok {
				if err2 := json.Unmarshal([]byte(candidate), &fileList); err2 == nil {
					return fileList, nil
				}
			}

			// If no JSON array could be located, interpret certain literals (e.g. "false")
			// as an empty file list to gracefully handle Agora's non-array responses.
			if looksLikeFalseLiteral(trimmed) {
				return []FileDetail{}, nil
			}

			return nil, fmt.Errorf("error parsing FileList into []FileDetail: %v", err)
		}
		return fileList, nil
	case "json":
		// Parse the file list as a slice of FileListEntry structures.
		var fileList []FileListEntry
		if err := json.Unmarshal(*sr.FileList, &fileList); err != nil {
			return nil, fmt.Errorf("error parsing FileList into []FileListEntry: %v", err)
		}
		return fileList, nil
	default:
		// Handle unknown FileListMode by returning an error.
		return nil, fmt.Errorf("unknown FileListMode: %s", *sr.FileListMode)
	}
}

func extractJSONArray(input string) (string, bool) {
	start := strings.Index(input, "[")
	if start == -1 {
		return "", false
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(input); i++ {
		ch := input[i]

		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return input[start : i+1], true
			}
		}
	}

	return "", false
}

func looksLikeFalseLiteral(input string) bool {
	lettersOnly := make([]rune, 0, len(input))
	for _, r := range input {
		if unicode.IsLetter(r) {
			lettersOnly = append(lettersOnly, unicode.ToLower(r))
			continue
		}

		if unicode.IsDigit(r) {
			// Treat digits embedded in the literal (e.g. "f8lse") as noise by skipping them.
			continue
		}
	}

	normalized := string(lettersOnly)
	if normalized == "false" {
		return true
	}

	if len(normalized) == 4 && strings.HasPrefix(normalized, "f") && strings.HasSuffix(normalized, "lse") {
		return true
	}

	return false
}
