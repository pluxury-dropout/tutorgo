// Package pdftool — шелл-ауты в poppler-utils (pdfinfo, pdftoppm).
// Шелл-аут, а не Go-библиотека: poppler переживает битые PDF лучше всего,
// а go-fitz тянет cgo и AGPL-лицензию MuPDF.
package pdftool

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"

	"tutorgo/models"
)

var pageSizeRe = regexp.MustCompile(`(?m)^Page\s+\d+\s+size:\s+([\d.]+)\s+x\s+([\d.]+)\s+pts`)

// ParseInfo выбирает габариты страниц из вывода pdfinfo. Страницы идут по
// порядку — номер из строки не читаем, доверяем порядку вывода.
func ParseInfo(out string) ([]models.PageSizePt, error) {
	matches := pageSizeRe.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("pdfinfo: не нашёл ни одной страницы")
	}
	sizes := make([]models.PageSizePt, len(matches))
	for i, m := range matches {
		w, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return nil, err
		}
		h, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return nil, err
		}
		sizes[i] = models.PageSizePt{W: w, H: h}
	}
	return sizes, nil
}

// Info возвращает габариты всех страниц документа.
func Info(ctx context.Context, path string) ([]models.PageSizePt, error) {
	// -l с запасом: pdfinfo печатает страницы только в запрошенном диапазоне.
	out, err := exec.CommandContext(ctx, "pdfinfo", "-f", "1", "-l", "1000000", path).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("pdfinfo: %w: %s", err, exitErr.Stderr)
		}
		return nil, fmt.Errorf("pdfinfo: %w", err)
	}
	return ParseInfo(string(out))
}

// RenderPage рендерит одну страницу (1-индексированную) в JPEG q85 и
// возвращает путь к файлу. Одна страница за вызов: память ограничена,
// а повтор джобы продолжает с места падения.
func RenderPage(ctx context.Context, pdfPath string, n, dpi int, outDir string) (string, error) {
	prefix := filepath.Join(outDir, fmt.Sprintf("p%d", n))
	cmd := exec.CommandContext(ctx, "pdftoppm",
		"-jpeg", "-jpegopt", "quality=85", "-r", strconv.Itoa(dpi),
		"-f", strconv.Itoa(n), "-l", strconv.Itoa(n), pdfPath, prefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("pdftoppm стр. %d: %w: %s", n, err, out)
	}
	// pdftoppm сам паддит номер в имени (p1-1.jpg / p1-01.jpg) — ловим глобом.
	matches, err := filepath.Glob(prefix + "-*.jpg")
	if err != nil || len(matches) != 1 {
		return "", fmt.Errorf("pdftoppm стр. %d: ожидал 1 файл, получил %d", n, len(matches))
	}
	return matches[0], nil
}
