package main

import (
	"fmt"
	"bufio"
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"runtime"
	"sync"
	"time"
	"flag"
	"os"
)

// THREADS represents the number of goroutines to be spawned for concurrent processing.
var THREADS int 
var REFLECT int = 0

// paramCheck represents a structure for holding URL and parameter information.
type paramCheck struct {
	url   string
	param string
}

func main() {
	// Initialize a scanner to read input.
	var sc *bufio.Scanner

	// Get information about the standard input.
	stat, _ := os.Stdin.Stat()


	// Define command-line flags.
	var inputFile string
	flag.StringVar(&inputFile, "i", "", "Input File Location")
	
	outputFile := "/tmp/reflxss-" + time.Now().Format("2006-01-02_15-04-05") + ".txt"
	flag.StringVar(&outputFile, "o", outputFile, "Output File Location")
	
	THREADS = runtime.NumCPU()*5
	flag.IntVar(&THREADS, "t", THREADS, "Number of Threads")

	flag.Parse()

	printBanner()

	// Initialize the scanner based on the input source.
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		sc = bufio.NewScanner(os.Stdin)
	} else if inputFile != "" {
		InFile, err := os.Open(inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
			os.Exit(1)
		}
		defer InFile.Close()
		sc = bufio.NewScanner(InFile)
	} else {
		fmt.Fprintln(os.Stderr, "No data available on standard input or first argument.")
		os.Exit(1)
	}

	// Open the output file to redirect standard output.
	OutFile, err := os.OpenFile(outputFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer OutFile.Close()

	fmt.Printf("▶ Output will be saved to: " + colorize(outputFile+"\n", "80"))

	// Configure the HTTP client to handle redirects appropriately.
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	// Initialize a channel for initial checks with a buffer size of THREADS.
	initialChecks := make(chan paramCheck, THREADS)

	// Create a pool of goroutines to perform initial checks concurrently.
	appendChecks := makePool(initialChecks, func(c paramCheck, output chan paramCheck) {
		reflected, err := checkReflected(c.url)
		if err != nil {
			return
		}

		if len(reflected) == 0 {
			return
		}

		for _, param := range reflected {
			output <- paramCheck{c.url, param}
		}
	})

	// Create a pool of goroutines to perform append checks concurrently.
	charChecks := makePool(appendChecks, func(c paramCheck, output chan paramCheck) {
		wasReflected, err := checkAppend(c.url, c.param, "x55hz33m")
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nERR %s on PARAM: %s\n", c.url, c.param)
			return
		}

		if wasReflected {
			output <- paramCheck{c.url, c.param}
		}
	})
	
	// Create a pool of goroutines to perform final checks concurrently.
	done := makePool(charChecks, func(c paramCheck, output chan paramCheck) {
		url_scan := []string{c.url, c.param}
		for _, char := range []string{"\"", "'", "<", ">"} {
			wasReflected, err := checkAppend(c.url, c.param, "pf1x"+char+"sf1x")
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nERR on [%s=%s] @ %s\n", c.url, c.param, char)
				continue
			}

			if wasReflected {
				url_scan = append(url_scan, char)
			}
		}
		if len(url_scan) >= 2 {

			REFLECT++

			fmt.Printf("\n"+colorize("%s = %v", "214"), url_scan[1], url_scan[2:])
			fmt.Printf("\n"+url_scan[0]+"\n")

			if _, err := fmt.Fprintf(OutFile, "%s = %v @ %s\n", url_scan[1], url_scan[2:], url_scan[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to file: %v\n", err)
			}
		}
	})

	for sc.Scan() {
		initialChecks <- paramCheck{url: sc.Text()}
	}

	close(initialChecks)
	<-done

	fmt.Printf("\n▶ Number of Reflected Paramters: " + colorize("%v", "80") + "\n", REFLECT)

}

func colorize(text, color string) string {
	return "\033[38;5;" + color + "m" + text + "\033[0m"
}

func printBanner() {

	bannerFormat := `
	 ___   ____  ____  _     _     __   __  
	| |_) | |_  | |_  | |   \ \_/ ( (%s ( (%s
	|_| \ |_|__ |_|   |_|__ /_/ \ _)_) _)_) 

				@xhzeem | v0.3				
	`
	banner := colorize(fmt.Sprintf(bannerFormat, "`","`"), "99")

	// Print to standard error
	fmt.Fprintln(os.Stderr, banner)
}

var transport = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: time.Second,
		DualStack: true,
	}).DialContext,
}

var httpClient = &http.Client{
	Transport: transport,
}

func checkReflected(targetURL string) ([]string, error) {

	out := make([]string, 0)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return out, err
	}

	// temporary. Needs to be an option
	req.Header.Add("User-Agent", "User-Agent: Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.100 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return out, err
	}
	if resp.Body == nil {
		return out, err
	}
	defer resp.Body.Close()

	// always read the full body so we can re-use the tcp connection
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return out, err
	}

	// nope (:
	if strings.HasPrefix(resp.Status, "3") {
		return out, nil
	}

	// also nope
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "html") {
		return out, nil
	}

	body := string(b)

	u, err := url.Parse(targetURL)
	if err != nil {
		return out, err
	}

	for key, vv := range u.Query() {
		for _, v := range vv {
			if !strings.Contains(body, v) {
				continue
			}

			out = append(out, key)
		}
	}

	return out, nil
}

func checkAppend(targetURL, param, suffix string) (bool, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return false, err
	}

	qs := u.Query()
	val := qs.Get(param)

	qs.Set(param, val+suffix)
	
	// Decoded Injection payload
	//	u.RawQuery, err = url.QueryUnescape(qs.Encode())
	u.RawQuery = qs.Encode()

	reflected, err := checkReflected(u.String())
	if err != nil {
		return false, err
	}

	for _, r := range reflected {
		if r == param {
			return true, nil
		}
	}

	return false, nil
}

type workerFunc func(paramCheck, chan paramCheck)

func makePool(input chan paramCheck, fn workerFunc) chan paramCheck {
	var wg sync.WaitGroup

	output := make(chan paramCheck)
	for i := 0; i < THREADS; i++ {
		wg.Add(1)
		go func() {
			for c := range input {
				fn(c, output)
			}
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(output)
	}()

	return output
}
