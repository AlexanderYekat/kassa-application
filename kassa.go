package main

import (
	"bytes"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

type Item struct {
	Code      string  `json:"code"`
	Article   string  `json:"article"`
	GroupCode string  `json:"groupCode"`
	IsGroup   bool    `json:"isGroup"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
}

type Employee struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

var (
	itemsMap map[string]Item
	sellers  []Employee
	plumbers []Employee
	useHTTPS = flag.Bool("https", true, "Использовать HTTPS (true) или HTTP (false)")
)

func main() {
	flag.Parse()

	itemsMap = readCSV("../goods_briefly.csv")
	sellers = readEmployees("../sellers.csv")
	plumbers = readEmployees("../plumbers.csv")

	//file, _ := os.Create("output.html")
	//defer file.Close()

	// Создаем новый мультиплексор
	mux := http.NewServeMux()

	// Регистрируем обработчики
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		lp := filepath.Join("templates", "index.html")
		tmpl := template.Must(template.ParseFiles(lp))
		tmpl.Execute(w, nil)
	})

	mux.HandleFunc("/api/product/", func(w http.ResponseWriter, r *http.Request) {
		code := filepath.Base(r.URL.Path)
		fmt.Println("поиск товара по коду:", code)
		w.Header().Set("Content-Type", "application/json")
		if item, ok := itemsMap[code]; ok {
			fmt.Println("товар найден:", item)
			json.NewEncoder(w).Encode(item)
		} else {
			http.Error(w, "Товар не найден", http.StatusNotFound)
		}
		fmt.Println("закончили поиск")
	})

	mux.HandleFunc("/sellers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sellers)
	})

	mux.HandleFunc("/plumbers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(plumbers)
	})

	mux.HandleFunc("/api/rukovoditel", func(w http.ResponseWriter, r *http.Request) {
		result, err := sendAPIRequest()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// Добавляем обработку статических файлов
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	if *useHTTPS {
		// Настройка TLS
		cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
		if err != nil {
			log.Fatalf("Ошибка загрузки сертификата и ключа: %v", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		// Создаем HTTPS сервер
		server := &http.Server{
			Addr:      ":8443",
			Handler:   mux,
			TLSConfig: tlsConfig,
		}

		log.Println("Сервер запущен на https://localhost:8443")
		log.Fatal(server.ListenAndServeTLS("", ""))
	} else {
		// Запускаем HTTP сервер
		log.Println("Сервер запущен на http://localhost:8080")
		log.Fatal(http.ListenAndServe(":8080", mux))
	}
}

func readCSV(filename string) map[string]Item {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.Comma = ';'
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	itemsMap := make(map[string]Item)
	for i, record := range records {
		if i == 0 { // Skip header
			continue
		}
		price, _ := strconv.ParseFloat(record[5], 64)
		item := Item{
			Code:      record[0],
			Article:   record[1],
			GroupCode: record[2],
			IsGroup:   record[3] == "1",
			Name:      record[4],
			Price:     price,
		}
		itemsMap[item.Code] = item
	}
	return itemsMap
}

func readEmployees(filename string) []Employee {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	var employees []Employee
	for i, record := range records {
		if i == 0 { // Skip header
			continue
		}
		employee := Employee{
			Code: record[0],
			Name: record[1],
		}
		employees = append(employees, employee)
	}

	// Sort by name
	sort.Slice(employees, func(i, j int) bool {
		return employees[i].Name < employees[j].Name
	})

	return employees
}

func sendAPIRequest() (map[string]interface{}, error) {
	jsonString := `{
		"key": "5rBkQICAed7fdJvmE6u0uOjsGMhNArmMLPqAxWrn",
		"username": "admin",
		"password": "admin",
		"action": "select",
		"entity_id": 25
	}`

	url := "https://rukovoditel.cloud/demo/3.5/api/rest.php?demo_id=3485"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(jsonString)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Println(string(body))

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
