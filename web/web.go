package web

import (
	"context"
	"embed"
	_ "embed"
	"io"
)

//go:embed static/*
var Files embed.FS

//go:embed static/css/styles.css
var TailwindCSS string

// RenderDashboard renders the administrative dashboard using templ.
func RenderDashboard(ctx context.Context, w io.Writer, data DashboardData) error {
	return Dashboard(data).Render(ctx, w)
}

// RenderLogin renders the admin login page using templ.
func RenderLogin(ctx context.Context, w io.Writer, data LoginData) error {
	return Login(data).Render(ctx, w)
}
