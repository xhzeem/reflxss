# reflxss
A basic tool to check for XSS vulnerabilities. It takes a list of URLs and checks if the parameter values appear in the response.

## Install

```bash
go install github.com/xhzeem/reflxss@latest
```

## Usage

Effortlessly scan for reflected XSS vulnerabilities in a list of URLs.
```bash
# Supply a file as the first argument
reflxss urls.txt

# Alternatively, pipe URLs through stdin
cat urls.txt | reflxss
```

<img width="837" alt="Screenshot 2024-01-21 at 6 41 49 PM" src="https://github.com/xhzeem/hxss/assets/34074156/99bc379b-04f4-487c-83da-30bfe62be3ba">

