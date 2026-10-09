package document

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"
)

func TestProcessPNG(
	t *testing.T,
) {

	imageData :=
		createTestPNG(t)

	doc, err :=
		Process(
			context.Background(),
			"receipt.png",
			imageData,
		)

	if err != nil {
		t.Fatal(err)
	}

	if doc.MIMEType != MIMEPNG {

		t.Fatalf(
			"expected %s, got %s",
			MIMEPNG,
			doc.MIMEType,
		)
	}

	if doc.PageCount != 1 {

		t.Fatalf(
			"expected 1 page, got %d",
			doc.PageCount,
		)
	}

	if len(doc.Pages) != 1 {

		t.Fatalf(
			"expected 1 page payload, got %d",
			len(doc.Pages),
		)
	}

	if doc.SHA256 == "" {

		t.Fatal(
			"expected SHA-256 hash",
		)
	}
}

func TestSHA256Stable(
	t *testing.T,
) {

	data :=
		[]byte(
			"same-file-content",
		)

	first :=
		CalculateSHA256(data)

	second :=
		CalculateSHA256(data)

	if first != second {

		t.Fatal(
			"same data should produce same SHA-256",
		)
	}
}

func TestDifferentContentDifferentSHA256(
	t *testing.T,
) {

	first :=
		CalculateSHA256(
			[]byte("file-one"),
		)

	second :=
		CalculateSHA256(
			[]byte("file-two"),
		)

	if first == second {

		t.Fatal(
			"different data should not produce the same SHA-256",
		)
	}
}

func TestUnsupportedDocument(
	t *testing.T,
) {

	_, err :=
		Process(
			context.Background(),
			"receipt.txt",
			[]byte(
				"this is a text document",
			),
		)

	if err == nil {

		t.Fatal(
			"expected unsupported document error",
		)
	}
}

func createTestPNG(
	t *testing.T,
) []byte {

	t.Helper()

	img :=
		image.NewRGBA(
			image.Rect(
				0,
				0,
				2,
				2,
			),
		)

	var buffer bytes.Buffer

	if err :=
		png.Encode(
			&buffer,
			img,
		); err != nil {

		t.Fatal(err)
	}

	return buffer.Bytes()
}
