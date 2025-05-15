package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

type AppSettings struct {
	Paths   []string `json:"paths"`
	Domains []string `json:"domains"`
}

var settings AppSettings

type URLStatus struct {
	URL    string `json:"url"`
	Status string `json:"status"`
}

func loadSettings() {
	file, err := os.Open("appsettings.json")
	if err != nil {
		panic(fmt.Sprintf("Error opening appsettings.json: %v", err))
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&settings); err != nil {
		panic(fmt.Sprintf("Error decoding appsettings.json: %v", err))
	}
}

func checkURL(url string) string {
	fmt.Println("CHECKING:", url)

	client := http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Println("RETURNING: ERROR:", err)
		return fmt.Sprintf("ERROR: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		fmt.Println("RETURNING: OK")
		return "OK"
	}
	fmt.Println("RETURNING: HTTP", resp.StatusCode)
	return fmt.Sprintf("HTTP %d", resp.StatusCode)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		http.Error(w, "Missing domain parameter", http.StatusBadRequest)
		return
	}

	var statuses []URLStatus
	for _, path := range settings.Paths {
		fullURL := fmt.Sprintf("https://%s%s", domain, path)
		status := checkURL(fullURL)
		statuses = append(statuses, URLStatus{
			URL:    fullURL,
			Status: status,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statuses)
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func frontendHandler(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>URL Health Checker</title>
	<style>
    body {
        font-family: Arial, sans-serif;
        background-color: #f1f5f0;
        color: #2f4f4f;
        margin: 0;
        padding: 20px;
        text-align: center;
    }

    h1 {
        color: #5a7d64;
        margin-bottom: 30px;
    }

    label, select {
        font-size: 16px;
        margin-bottom: 20px;
    }

    table {
        margin: 20px auto;
        border-collapse: collapse;
        width: 80%;
        max-width: 800px;
        background-color: #ffffff;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    }

    th, td {
        border: 1px solid #b2c2b4;
        padding: 12px 20px;
        font-size: 14px;
    }

    th {
        background-color: #d9e8dd;
        color: #344e41;
    }

    tr:nth-child(even) {
        background-color: #f6f9f7;
    }

    td {
        word-break: break-all;
    }

    select {
        padding: 5px 10px;
        border-radius: 4px;
        border: 1px solid #b2c2b4;
    }
	</style>
</head>
<body>
    <h1>URL Health Checker</h1>
    <label for="domainSelect">Select Domain:</label>
    <select id="domainSelect"></select>
    <table border="1">
        <thead>
            <tr>
                <th>URL</th>
                <th>Status</th>
            </tr>
        </thead>
        <tbody id="statusTable"></tbody>
    </table>

<script>
    let domains = [];

    async function loadConfig() {
        const response = await fetch('/config');
        const config = await response.json();
        domains = config.domains;

        const select = document.getElementById('domainSelect');
        domains.forEach(domain => {
            const option = document.createElement('option');
            option.value = domain;
            option.textContent = domain;
            select.appendChild(option);
        });

        // default domain status
        if (domains.length > 0) {
            select.value = domains[0];
            fetchStatus();
        }

        // change listener for switching domains
        select.addEventListener('change', fetchStatus);
    }

    function getStatusColor(status) {
        if (status === 'OK') return 'green';
        if (status.startsWith('HTTP 4')) return 'orange';
        if (status.startsWith('HTTP 5')) return 'red';
        if (status.startsWith('ERROR')) return 'gray';
        return 'black';
    }

    async function fetchStatus() {
        const selectedDomain = document.getElementById('domainSelect').value;
        if (!selectedDomain) return;

        const response = await fetch('/status?domain=' + selectedDomain);
        const statuses = await response.json();

        const table = document.getElementById('statusTable');
        table.innerHTML = '';

        statuses.forEach(item => {
            const row = document.createElement('tr');

            const urlCell = document.createElement('td');
            urlCell.textContent = item.url;

            const statusCell = document.createElement('td');
            statusCell.textContent = item.status;
            statusCell.style.color = getStatusColor(item.status);

            row.appendChild(urlCell);
            row.appendChild(statusCell);
            table.appendChild(row);
        });
    }

    document.addEventListener('DOMContentLoaded', loadConfig);
</script>

<button onclick="fetchStatus()" style="margin-top: 20px; padding: 10px 20px; font-size: 16px;">
    Refresh
</button>

</body>
</html>`
	fmt.Fprint(w, html)
}

func openBrowser(url string) {
	//for opening browser immediately
	var err error

	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = exec.Command("xdg-open", url).Start()
	}

	if err != nil {
		fmt.Println("Failed to open browser:", err)
	}
}

func main() {
	loadSettings()

	http.HandleFunc("/", frontendHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/config", configHandler)

	fmt.Println("Server running on http://localhost:8080")

	go func() {
		time.Sleep(1 * time.Second)
		openBrowser("http://localhost:8080")
	}()

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Failed to start server:", err)
	}
}
