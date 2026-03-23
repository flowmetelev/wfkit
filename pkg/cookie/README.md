# Cookie Package

This package provides utility functions to extract cookies from various web browsers across macOS, Linux, and Windows.

It supports extracting cookies from:
- **Chromium-based browsers:** Chrome, Brave, Edge, Vivaldi, Opera, Chromium, Arc, Opera GX, Octo Browser
- **Firefox-based browsers:** Firefox, Zen, LibreWolf
- **Safari** (macOS only)

The extraction is powered natively by the `kooky` library, removing the need for external scripts or heavy compiled extractors.

## Usage

To use the cookie extractor in your Go project:

1. Import the `cookie` package.
2. Create a configuration using `cookie.DefaultConfig()` (or customize it as needed).
3. Call `cookie.GetAllCookies(domains, config)` to extract cookies for specific domains.

### Example

```go
package main

import (
	"fmt"
	"log"
	"wfkit/pkg/cookie"
)

func main() {
	config := cookie.DefaultConfig()
	
	// Extract cookies specifically for "github.com"
	cookies, err := cookie.GetAllCookies([]string{"github.com"}, config)
	if err != nil {
		log.Fatalf("Error getting cookies: %v", err)
	}
	
	for _, c := range cookies {
		fmt.Printf("Cookie: %s=%s (Domain: %s)\n", c.Name, c.Value, c.Domain)
	}
}
```

## Running Tests

To run the tests for the `pkg/cookie` module:

```bash
go test -v ./pkg/cookie/...
```
