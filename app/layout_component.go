package app

import (
	"context"
	"io"

	"github.com/a-h/templ"
)

func LayoutWithAssets(title string, styles []string, scripts []string, body templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		write := func(s string) error {
			_, err := io.WriteString(w, s)
			return err
		}

		if err := write("<!DOCTYPE html><html lang=\"en\"><head><meta charset=\"UTF-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\"><title>"); err != nil {
			return err
		}
		if err := write(templ.EscapeString(title)); err != nil {
			return err
		}
		if err := write("</title><link rel=\"preload\" href=\"/fonts/Inter-400.ttf\" as=\"font\" type=\"font/ttf\" crossorigin><link rel=\"preload\" href=\"/fonts/Inter-500.ttf\" as=\"font\" type=\"font/ttf\" crossorigin><link rel=\"preload\" href=\"/fonts/Inter-600.ttf\" as=\"font\" type=\"font/ttf\" crossorigin><link rel=\"preload\" href=\"/fonts/Inter-700.ttf\" as=\"font\" type=\"font/ttf\" crossorigin><link rel=\"stylesheet\" href=\"/fonts.css\"><link rel=\"stylesheet\" href=\"/styles.css\">"); err != nil {
			return err
		}
		for _, href := range styles {
			if err := write("<link rel=\"stylesheet\" href=\""); err != nil {
				return err
			}
			if err := write(templ.EscapeString(href)); err != nil {
				return err
			}
			if err := write("\">"); err != nil {
				return err
			}
		}
		if err := write("</head><body class=\"is-loading\">"); err != nil {
			return err
		}
		if body != nil {
			if err := body.Render(ctx, w); err != nil {
				return err
			}
		}
		if err := write("<script src=\"/app.js\" defer></script>"); err != nil {
			return err
		}
		for _, src := range scripts {
			if err := write("<script src=\""); err != nil {
				return err
			}
			if err := write(templ.EscapeString(src)); err != nil {
				return err
			}
			if err := write("\" defer></script>"); err != nil {
				return err
			}
		}
		return write("</body></html>")
	})
}
