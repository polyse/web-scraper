package module

import (
	"bufio"
	"encoding/json"
	"github.com/polyse/web-scraper/internal/connection"
	zl "github.com/rs/zerolog/log"
	"os"
)

type Module struct {
	Conn    *connection.Connection
	outPath string
}

func NewModule(filePath string, outPath string, conn *connection.Connection) (*Module, error) {
	m := &Module{
		Conn:    conn,
		outPath: outPath,
	}
	files := m.ReadFilePath(filePath)
	m.Conn.Files = files
	return m, nil
}

func (m *Module) ReadFilePath(filePath string) []string {
	var listOfFiles = []string{}
	file, err := os.Open(filePath)
	defer file.Close()
	if err != nil {
		zl.Fatal().Err(err).
			Msgf("Can't open input file %v", filePath)
	}
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		listOfFiles = append(listOfFiles, scanner.Text())
	}
	zl.Debug().Msgf("IN: %v", listOfFiles)
	return listOfFiles
}

func (m *Module) WriteResult() {
	file, _ := os.OpenFile(m.outPath, os.O_CREATE, os.ModePerm)
	defer file.Close()

	encoder := json.NewEncoder(file)
	err := encoder.Encode(&m.Conn.SitesInfo)

	if err != nil {
		zl.Fatal().Err(err).
			Msg("Can't write json in file")
	}
}
