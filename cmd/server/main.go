package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kubevalet/kubevalet/internal/api"
	"github.com/kubevalet/kubevalet/internal/auth"
	"github.com/kubevalet/kubevalet/internal/config"
	"github.com/kubevalet/kubevalet/internal/db"
	"github.com/kubevalet/kubevalet/internal/k8s"
	"github.com/kubevalet/kubevalet/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Kubernetes client
	k8sClient, err := k8s.New(cfg.KubeconfigPath)
	if err != nil {
		log.Fatalf("init k8s client: %v", err)
	}
	log.Printf("connected to cluster %q (%s)", cfg.ClusterName, k8sClient.RestConfig.Host)

	// PostgreSQL
	if cfg.PostgresDSN == "" {
		log.Fatal("POSTGRES_DSN is required")
	}
	ctx := context.Background()
	pool, err := db.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("init postgres: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// Seed initial admin user from env vars (only when table is empty)
	if cfg.AdminUsername != "" && cfg.AdminPassword != "" {
		hash, err := auth.HashPassword(cfg.AdminPassword)
		if err != nil {
			log.Fatalf("hash admin password: %v", err)
		}
		if err := db.SeedAdmin(ctx, pool, cfg.AdminUsername, hash); err != nil {
			log.Fatalf("seed admin: %v", err)
		}
		log.Printf("admin user %q ready", cfg.AdminUsername)
	}

	// HTTP server
	gin.SetMode(cfg.GinMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	h := api.New(k8sClient, cfg, pool)
	v1 := r.Group("/api/v1")
	h.RegisterPublic(v1)
	h.RegisterProtected(v1)

	// Serve embedded frontend (Vue SPA)
	distFS, err := fs.Sub(web.FS, "dist")
	if err != nil {
		log.Fatalf("embed frontend: %v", err)
	}
	r.NoRoute(spaHandler(http.FS(distFS)))

	log.Printf("kubevalet listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// spaHandler serves static files and falls back to index.html for client-side routing.
func spaHandler(fsys http.FileSystem) gin.HandlerFunc {
	fileServer := http.FileServer(fsys)
	return func(c *gin.Context) {
		// Check if the file exists in the FS
		f, err := fsys.Open(c.Request.URL.Path)
		if err != nil {
			// Not found → serve index.html for Vue Router to handle
			c.Request.URL.Path = "/"
		} else {
			f.Close()
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}
