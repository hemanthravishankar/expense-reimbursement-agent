package document

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	MIMEJPEG = "image/jpeg"
	MIMEPNG  = "image/png"
	MIMEPDF  = "application/pdf"
)

type Document struct {
	OriginalName string   `json:"original_name"`
	MIMEType     string   `json:"mime_type"`
	SHA256       string   `json:"sha256"`
	PageCount    int      `json:"page_count"`
	Pages        [][]byte `json:"-"`
}

func Process(
	ctx context.Context,
	filename string,
	data []byte,
) (*Document, error) {

	if len(data) == 0 {
		return nil, fmt.Errorf(
			"document is empty",
		)
	}

	mimeType :=
		http.DetectContentType(data)

	doc := &Document{
		OriginalName: filepath.Base(filename),
		MIMEType:     mimeType,
		SHA256:       CalculateSHA256(data),
	}

	switch mimeType {

	case MIMEJPEG, MIMEPNG:

		doc.Pages = [][]byte{
			data,
		}

	case MIMEPDF:

		pages, err :=
			renderPDF(
				ctx,
				data,
			)

		if err != nil {
			return nil, err
		}

		doc.Pages =
			pages

	default:

		return nil, fmt.Errorf(
			"unsupported document type %q for file %q",
			mimeType,
			filename,
		)
	}

	doc.PageCount =
		len(doc.Pages)

	if doc.PageCount == 0 {
		return nil, fmt.Errorf(
			"document contains no processable pages",
		)
	}

	return doc, nil
}

func CalculateSHA256(
	data []byte,
) string {

	sum :=
		sha256.Sum256(data)

	return hex.EncodeToString(
		sum[:],
	)
}

func renderPDF(
	ctx context.Context,
	data []byte,
) ([][]byte, error) {

	if _, err :=
		exec.LookPath("pdftoppm"); err != nil {

		return nil, fmt.Errorf(
			"PDF support requires pdftoppm; install Poppler first: %w",
			err,
		)
	}

	tempDir, err :=
		os.MkdirTemp(
			"",
			"expense-agent-pdf-*",
		)

	if err != nil {
		return nil, fmt.Errorf(
			"create PDF temp directory: %w",
			err,
		)
	}

	defer os.RemoveAll(
		tempDir,
	)

	inputPath :=
		filepath.Join(
			tempDir,
			"input.pdf",
		)

	if err := os.WriteFile(
		inputPath,
		data,
		0600,
	); err != nil {

		return nil, fmt.Errorf(
			"write temporary PDF: %w",
			err,
		)
	}

	outputPrefix :=
		filepath.Join(
			tempDir,
			"page",
		)

	cmd :=
		exec.CommandContext(
			ctx,
			"pdftoppm",
			"-png",
			"-r",
			"150",
			inputPath,
			outputPrefix,
		)

	output, err :=
		cmd.CombinedOutput()

	if err != nil {

		return nil, fmt.Errorf(
			"render PDF pages: %w: %s",
			err,
			strings.TrimSpace(
				string(output),
			),
		)
	}

	files, err :=
		filepath.Glob(
			outputPrefix + "-*.png",
		)

	if err != nil {
		return nil, fmt.Errorf(
			"find rendered PDF pages: %w",
			err,
		)
	}

	sort.Slice(
		files,
		func(i, j int) bool {

			return pageNumber(files[i]) <
				pageNumber(files[j])
		},
	)

	pages :=
		make(
			[][]byte,
			0,
			len(files),
		)

	for _, path := range files {

		pageData, err :=
			os.ReadFile(path)

		if err != nil {

			return nil, fmt.Errorf(
				"read rendered PDF page %s: %w",
				path,
				err,
			)
		}

		pages =
			append(
				pages,
				pageData,
			)
	}

	return pages, nil
}

func pageNumber(
	path string,
) int {

	base :=
		filepath.Base(path)

	base =
		strings.TrimSuffix(
			base,
			filepath.Ext(base),
		)

	index :=
		strings.LastIndex(
			base,
			"-",
		)

	if index == -1 {
		return 0
	}

	number, err :=
		strconv.Atoi(
			base[index+1:],
		)

	if err != nil {
		return 0
	}

	return number
}
