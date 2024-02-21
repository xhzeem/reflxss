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
	"context"
    "github.com/chromedp/chromedp"
)

var THREADS int 
var REFLECT int = 0
var DOMdelay int
var userAgent string

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

	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 12_2_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/111.0.0.0 Safari/537.36"
	flag.StringVar(&userAgent, "ua", userAgent, "User Agent Header")

	var checkDOM bool
	flag.BoolVar(&checkDOM, "dom", false, "Check the DOM response instead")	

	DOMdelay = 0
	flag.IntVar(&DOMdelay, "delay", DOMdelay, "Seconds to wait before fetching the DOM")
	
	THREADS = runtime.NumCPU() * 5
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
		
		// reflected, err := checkReflected(c.url)
		// if err != nil {
		// 	return
		// }

		var reflected []string
		
		targetURL, err := url.Parse(c.url)
		if err != nil {
			return
		}

		for key := range targetURL.Query() {
			reflected = append(reflected, key)
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

		var wasReflected bool

		if checkDOM {
			// Use the chromedp based DOM check
			wasReflected, err = checkDOMResponse(c.url, c.param, "xhz33m")
		} else {
			// Use the existing HTTP check
			wasReflected, err = checkAppend(c.url, c.param, "xhz33m")
		}

		// Handling reflection check
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

		url_scan := c.url
		url_param := c.param
		ref_chars := []string{}
		for _, char := range []string{"\"", "'", "<", ">"} {
			var wasReflected bool

			if checkDOM {
				// Use the chromedp based DOM check
				wasReflected, err = checkDOMResponse(c.url, c.param, "pf1x"+char+"sf1x")
			} else {
				// Use the existing HTTP check
				wasReflected, err = checkAppend(c.url, c.param, "pf1x"+char+"sf1x")
			}

			if err != nil {
				fmt.Fprintf(os.Stderr, "\nERR on [%s=%s] @ %s\n", c.url, c.param, char)
				continue
			}

			if wasReflected {
				ref_chars = append(ref_chars, char)
			}
		}
		if len(ref_chars) > 0 {
			REFLECT++
			
			fmt.Printf("\n" + colorize("%s = %v", "214") + "\n%s\n", url_param, ref_chars, url_scan)

			if _, err := fmt.Fprintf(OutFile, "%s = %v @ %s\n", url_param, ref_chars, url_scan); err != nil {
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

				@xhzeem | v0.4				
	`
	banner := colorize(fmt.Sprintf(bannerFormat, "`","`"), "99")

	// Print to standard error
	fmt.Fprintln(os.Stderr, banner)
}

var transport = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	DialContext: (&net.Dialer{

		// Proxy: http.ProxyURL(&url.URL{
		// 	Scheme: "http", 
		// 	Host:   "127.0.0.1:8080",
		// }),

		Timeout:   15 * time.Second,
		KeepAlive: time.Second,
		DualStack: true,
		
	}).DialContext,
}

var httpClient = &http.Client{
	Transport: transport,
}

func checkDOMResponse(targetURL, param string, suffix string) (bool, error) {
	
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return false, err
	}

	qs := parsedURL.Query()
	val := qs.Get(param)

	qs.Set(param, val+suffix)
	parsedURL.RawQuery = qs.Encode()

	// Create an allocator context for chromedp to use our custom options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("proxy-server","http://127.0.0.1:8080"),
		chromedp.UserAgent(userAgent),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(allocCtx, time.Duration(DOMdelay+15) * time.Second)
	defer cancel()

    // Now create a chromedp context
    chromedpCtx, chromedpCancel := chromedp.NewContext(ctx)
    defer chromedpCancel()

	// Navigate to the URL and wait if necessary before extracting the entire DOM
	var body string
	tasks := chromedp.Tasks{
		chromedp.Navigate(parsedURL.String()),
	}

	if DOMdelay > 0 {
		tasks = append(tasks, chromedp.Sleep(time.Duration(DOMdelay) * time.Second))
	}
	tasks = append(tasks, chromedp.OuterHTML("html", &body, chromedp.ByQuery))

	err = chromedp.Run(chromedpCtx, tasks)
	if err != nil {
		return false, err
	}

	// Check if the parameter value is reflected in the DOM
	reflected := strings.Contains(body, suffix)

	return reflected, nil
}


func checkReflected(targetURL string) ([]string, error) {

	out := make([]string, 0)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return out, err
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return out, err
	}
	if resp.Body == nil {
		return out, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return out, err
	}

	// Check if the response is redirect 
	if strings.HasPrefix(resp.Status, "3") {
		return out, nil
	}

	// Confirm the MiME response is HTML
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "html") {
		return out, nil
	}

	u, err := url.Parse(targetURL)
	if err != nil {
		return out, err
	}

	for key, vv := range u.Query() {
		for _, v := range vv {
			if !strings.Contains(string(body), v) {
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
	// u.RawQuery, err = url.QueryUnescape(qs.Encode())
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
