package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

func colorize(text, color string) string {
	return "\033[38;5;" + color + "m" + text + "\033[0m"
}

func printBanner() {

	bannerFormat := `
	 ___   ____  ____  _     _     __   __  
	| |_) | |_  | |_  | |   \ \_/ ( (%s ( (%s
	|_| \ |_|__ |_|   |_|__ /_/ \ _)_) _)_) 

				@xhzeem | v0.2			
	`
	banner := colorize(fmt.Sprintf(bannerFormat, "`","`"), "99")

	// Print to standard error
	fmt.Fprintln(os.Stderr, banner)
}

type paramCheck struct {
	url   string
	param string
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

func main() {

	var sc *bufio.Scanner
	var URLsFile string
	stat, _ := os.Stdin.Stat()

	if (stat.Mode() & os.ModeCharDevice) == 0 {
		sc = bufio.NewScanner(os.Stdin)
	} else if (len(os.Args) > 1) {
		URLsFile = os.Args[1]
		file, err := os.Open(URLsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		sc = bufio.NewScanner(file)
	} else {
		fmt.Fprintln(os.Stderr, "No data available on standard input or first argument.")
		os.Exit(1)
	}

	printBanner()

	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	initialChecks := make(chan paramCheck, 40)

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

	done := makePool(charChecks, func(c paramCheck, output chan paramCheck) {
		output_of_url := []string{c.url, c.param}
		for _, char := range []string{"\"", "'", "<", ">"} {
			wasReflected, err := checkAppend(c.url, c.param, "pf1x"+char+"sf1x")
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nERR on [%s=%s] @ %s\n", c.url, c.param, char)
				continue
			}

			if wasReflected {
				output_of_url = append(output_of_url, char)
			}
		}
		if len(output_of_url) >= 2 {
			fmt.Printf("\n"+colorize("%s = %v", "214"), output_of_url[1], output_of_url[2:])
			fmt.Printf("\n"+output_of_url[0]+"\n")
		}
	})

	for sc.Scan() {
		initialChecks <- paramCheck{url: sc.Text()}
	}

	close(initialChecks)
	<-done
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
	u.RawQuery, err = url.QueryUnescape(qs.Encode())

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
	for i := 0; i < 40; i++ {
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
