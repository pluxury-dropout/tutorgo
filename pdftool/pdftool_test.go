package pdftool

import "testing"

// Реальный формат вывода pdfinfo -f 1 -l N: строки "Page N size: W x H pts (…)".
const sampleInfo = `Title:          Учебник
Producer:       LibreOffice
Pages:          3
Page    1 size: 612 x 792 pts (letter)
Page    1 rot:  0
Page    2 size: 595.28 x 841.89 pts (A4)
Page    2 rot:  0
Page    3 size: 841.89 x 595.28 pts (A4)
Page    3 rot:  90
File size:      12345 bytes`

func TestParseInfo(t *testing.T) {
	sizes, err := ParseInfo(sampleInfo)
	if err != nil {
		t.Fatal(err)
	}
	if len(sizes) != 3 {
		t.Fatalf("want 3 pages, got %d", len(sizes))
	}
	if sizes[0].W != 612 || sizes[0].H != 792 {
		t.Errorf("page 1: got %+v", sizes[0])
	}
	if sizes[1].W != 595.28 {
		t.Errorf("page 2 W: got %v", sizes[1].W)
	}
	// альбомная страница — ширина больше высоты, порядок не путаем
	if sizes[2].W != 841.89 || sizes[2].H != 595.28 {
		t.Errorf("page 3: got %+v", sizes[2])
	}
}

func TestParseInfoNoPages(t *testing.T) {
	if _, err := ParseInfo("Producer: x\n"); err == nil {
		t.Fatal("want error for output without pages")
	}
}
