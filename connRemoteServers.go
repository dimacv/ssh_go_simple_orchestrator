package main

import (
	"bufio"
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func connectingRemoteServers(host serverConfig, commands []string, outchan chan string, errchan chan string) {
	defer wg.Done()

	var signer ssh.Signer

	// Создание файла в подпапке
	filePath := filepath.Join(subFolderPath, host.IP+".log")
	logHost, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logHost.Close()

	buffer := new(bytes.Buffer)

	fmt.Fprintf(buffer, "\n*****************************************************************************")
	fmt.Fprintf(buffer, "\n")
	fmt.Fprintf(buffer, "________________  HOST - %s  ____________________ \n\n", host.IP)

	// Загрузка закрытого ключа для аутентификации
	key, err := os.ReadFile(host.KeyPath)
	if err != nil {
		logError("Не удалось прочитать закрытый ключ: ", err, host.IP, errchan, outchan, logHost)
		return
	}

	// Загрузка приватного ключа без парольной фразы
	if len(passphrase) == 0 {
		signer, err = ssh.ParsePrivateKey(key)
	} else {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(key, passphrase)
	}
	if err != nil {
		logError("Не удалось разобрать закрытый ключ: ", err, host.IP, errchan, outchan, logHost)
		return
	}

	// Создание структуры конфигурации SSH
	conf := &ssh.ClientConfig{
		User: host.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Не использовать в продакшене!
	}

	// Подключение к серверу
	connection, err := ssh.Dial("tcp", host.IP+":"+host.Port, conf)
	if err != nil {
		logError("Не удалось подключиться: ", err, host.IP, errchan, outchan, logHost)
		return
	}
	defer connection.Close()

	// Создание сессии
	session, err := connection.NewSession()
	if err != nil {
		logError("Не удалось создать сессию: ", err, host.IP, errchan, outchan, logHost)
		return
	}
	defer session.Close()

	// Переменная для хранения результата
	var splicedCommands string

	for i, command := range commands {
		// Добавляем элемент к результату
		splicedCommands += command
		// Если это не последний элемент, добавляем разделитель
		if i < len(commands)-1 {
			splicedCommands += ";"
		}
	}

	// Выполнение команды на удаленном сервере
	output, err := session.CombinedOutput(splicedCommands)
	if err != nil {
		log.Printf("Не удалось выполнить команды: %v\n", err)
		return
	}

	fmt.Fprintf(buffer, string(output))

	// Пишем вывод напрямую в файл журнала для данной горутины
	fmt.Fprintf(logHost, buffer.String())

	fmt.Fprintf(buffer, "\n*****************************************************************************\n")
	outchan <- fmt.Sprintln(buffer.String())
}

func containsOnlyWhitespaceOrEmpty(s string) bool {
	if len(strings.TrimSpace(s)) == 0 {
		return true // строка пуста
	}
	return false
}

func parseServerConfigLine(line string) (serverConfig, error) {
	var config serverConfig
	if strings.HasPrefix(line, "#") || containsOnlyWhitespaceOrEmpty(line) {
		return config, fmt.Errorf("Ignored comment line")
	}

	var fields []string

	if strings.Contains(line, ",") {
		fields = strings.Split(line, ",")
	} else if strings.Contains(line, ";") || len(line) > 0 {
		fields = strings.Split(line, ";")
	} else {
		return config, fmt.Errorf("Invalid configuration format")
	}

	config.IP = strings.TrimSpace(fields[0])

	if len(fields) <= 2 && username != "" && sshKey != "" && port != "" {
		config.Username = username
		config.KeyPath = sshKey
		config.Port = port
	} else if len(fields) <= 3 && sshKey != "" && port != "" {
		username = strings.TrimSpace(fields[1])
		config.Username = strings.TrimSpace(fields[1])
		config.KeyPath = sshKey
		config.Port = port
	} else if len(fields) <= 4 && port != "" {
		username = strings.TrimSpace(fields[1])
		config.Username = strings.TrimSpace(fields[1])
		sshKey = strings.TrimSpace(fields[2])
		config.KeyPath = strings.TrimSpace(fields[2])
		config.Port = port
	} else {
		username = strings.TrimSpace(fields[1])
		config.Username = strings.TrimSpace(fields[1])
		sshKey = strings.TrimSpace(fields[2])
		config.KeyPath = strings.TrimSpace(fields[2])
		if len(fields) <= 4 || containsOnlyWhitespaceOrEmpty(fields[3]) {
			port = "22"
			config.Port = "22"
		} else {
			port = strings.TrimSpace(fields[3])
			config.Port = strings.TrimSpace(fields[3])
		}
	}

	return config, nil
}

func getCommandsFromConfig(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var commands []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		command := scanner.Text()
		commands = append(commands, command)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return commands, nil
}

func logError(s string, err error, host string, errchan chan string, outchan chan string, logHost *os.File) {
	message := fmt.Sprintf("\nERROR => ************************************************************************ \n"+
		"  ERROR  ON   HOST  - %s   \n\n"+
		"  ERROR -> %s  %v \n "+
		"_______________________________________________________________________________ \n \n ", host, s, err)

	// Пишем вывод напрямую в файл журнала для данной горутины
	fmt.Fprintf(logHost, message)

	errchan <- message
	outchan <- message
}
