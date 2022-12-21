package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var db *sql.DB

func init() {
	var err error
	db, err = sql.Open("sqlite3", "./endpoints.db")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS endpoints (id INTEGER PRIMARY KEY AUTOINCREMENT, url TEXT NOT NULL, interval INTEGER NOT NULL, last_checked INTEGER NOT NULL, status INTEGER NOT NULL)`);
	err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func monitorEndpoint(url string, interval int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	for range ticker.C {
		resp, err := http.Get(url)
		if err != nil {
			fmt.Println(err)
			continue
		}
		status := resp.StatusCode
		if _, err := db.Exec("INSERT INTO endpoints (url, interval, last_checked, status) VALUES (?, ?, ?, ?)", url, interval, time.Now().Unix(), status); err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Printf("%s - %d\n", url, status)
	}
}

func monitorEndpoints(cmd *cobra.Command, args []string) {
	rows, err := db.Query("SELECT url, interval FROM endpoints")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer rows.Close()
	var wg sync.WaitGroup
	for rows.Next() {
		var url string
		var interval int
		if err := rows.Scan(&url, &interval); err != nil {
			fmt.Println(err)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			monitorEndpoint(url, interval)
		}()
	}
	wg.Wait()
}

func addEndpoint(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: endpoint add <url> <interval>")
		return
	}
	url := args[0]
	interval, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Println("Interval must be a number")
		return
	}
	if _, err := db.Exec("INSERT INTO endpoints (url, interval, last_checked, status) VALUES (?, ?, ?, ?)", url, interval, time.Now().Unix(), 0); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Endpoint added")
}

func listEndpoints(cmd *cobra.Command, args []string) {
	rows, err := db.Query("SELECT url, interval, MAX(last_checked) AS last_checked, status FROM endpoints GROUP BY url ORDER BY id ASC")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer rows.Close()
	data := [][]string{}
	for rows.Next() {
		var url string
		var interval int
		var lastChecked int
		var status int
		if err := rows.Scan(&url, &interval, &lastChecked, &status); err != nil {
			fmt.Println(err)
			continue
		}
		lastCheckedStr := time.Unix(int64(lastChecked), 0).Format("2006-01-02 15:04:05")
		data = append(data, []string{url, strconv.Itoa(interval), lastCheckedStr, strconv.Itoa(status)})
	}
	printTable(data)
}

func removeEndpoint(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: endpoint remove <url>")
		return
	}
	url := args[0]
	if _, err := db.Exec("DELETE FROM endpoints WHERE url = ?", url); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Endpoint removed")
}

func printTable(data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"URL", "Interval (seconds)", "Last Checked", "Status"})
	for _, v := range data {
		table.Append(v)
	}
	table.Render()
}

func main() {
	rootCmd := &cobra.Command{Use: "endpoint"}
	rootCmd.AddCommand(&cobra.Command{
		Use:   "add",
		Short: "Add an endpoint to monitor",
		Run:   addEndpoint,
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all monitored endpoints",
		Run:   listEndpoints,
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start monitoring endpoints",
		Run:   monitorEndpoints,
	})
	rootCmd.AddCommand(&cobra.Command{
		Use:   "remove",
		Short: "Remove an endpoint from the list of monitored endpoints",
		Run:   removeEndpoint,
	})
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
