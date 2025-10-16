package layout

import (
	"context"
	"io"

	"github.com/a-h/templ"
)

func LayoutWithAssets(title string, styles []string, body templ.Component) templ.Component {
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
		if err := write("</title><link rel=\"preload\" href=\"/fonts/Inter-400.ttf\" as=\"font\" type=\"font/ttf\" crossorigin><link rel=\"preload\" href=\"/fonts/Inter-500.ttf\" as=\"font\" type=\"font/ttf\" crossorigin><link rel=\"preload\" href=\"/fonts/Inter-600.ttf\" as=\"font\" type=\"font/ttf\" crossorigin><link rel=\"preload\" href=\"/fonts/Inter-700.ttf\" as=\"font\" type=\"font/ttf\" crossorigin>"); err != nil {
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
		if err := write("</head><body hx-boost=\"true\" hx-ext=\"json-enc\">"); err != nil {
			return err
		}
		if body != nil {
			if err := body.Render(ctx, w); err != nil {
				return err
			}
		}
		htmxScript := "<script src=\"https://cdn.jsdelivr.net/npm/htmx.org@2.0.7/dist/htmx.min.js\" integrity=\"sha384-ZBXiYtYQ6hJ2Y0ZNoYuI+Nq5MqWBr+chMrS/RkXpNzQCApHEhOt2aY8EJgqwHLkJ\" crossorigin=\"anonymous\" defer></script>"
		jsonEncScript := "<script src=\"https://cdn.jsdelivr.net/npm/htmx.org@2.0.7/dist/ext/json-enc.js\" crossorigin=\"anonymous\" defer></script>"
		if err := write(htmxScript); err != nil {
			return err
		}
		if err := write(jsonEncScript); err != nil {
			return err
		}
		return write("</body></html>")
	})
}
