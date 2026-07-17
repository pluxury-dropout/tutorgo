package pdftool

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Минимальный двухстраничный PDF без xref-таблицы: poppler реконструирует её
// сам (в stderr будет Syntax Warning — это норма). Вторая страница A4.
const tinyPdf = `%PDF-1.4
1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj
2 0 obj << /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >> endobj
3 0 obj << /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >> endobj
4 0 obj << /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >> endobj
trailer << /Root 1 0 R /Size 5 >>
`

func TestSmokePoppler(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm не установлен")
	}
	if _, err := exec.LookPath("pdfinfo"); err != nil {
		t.Skip("pdfinfo не установлен")
	}
	dir := t.TempDir()
	pdf := filepath.Join(dir, "tiny.pdf")
	if err := os.WriteFile(pdf, []byte(tinyPdf), 0o644); err != nil {
		t.Fatal(err)
	}

	sizes, err := Info(context.Background(), pdf)
	if err != nil {
		t.Fatal(err)
	}
	if len(sizes) != 2 || sizes[0].W != 612 || sizes[1].H != 842 {
		t.Fatalf("got %+v", sizes)
	}

	jpeg, err := RenderPage(context.Background(), pdf, 2, 72, dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(jpeg)
	if err != nil || st.Size() == 0 {
		t.Fatalf("пустой или отсутствующий jpeg: %v", err)
	}
}
