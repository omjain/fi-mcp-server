package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/epifi/fi-mcp-lite/middlewares"
	"github.com/epifi/fi-mcp-lite/pkg"
)

var authMiddleware *middlewares.AuthMiddleware

func main() {
	authMiddleware = middlewares.NewAuthMiddleware()
	s := server.NewMCPServer(
		"Hackathon MCP",
		"0.1.0",
		server.WithInstructions("..."), // keep your long instructions as-is
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
		server.WithToolHandlerMiddleware(authMiddleware.AuthMiddleware),
	)

	// Register tools
	for _, tool := range pkg.ToolList {
		s.AddTool(mcp.NewTool(tool.Name, mcp.WithDescription(tool.Description)), dummyHandler)
	}

	// HTTP handlers
	httpMux := http.NewServeMux()
	httpMux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	streamableServer := server.NewStreamableHTTPServer(s, server.WithEndpointPath("/stream"))
	httpMux.Handle("/mcp/", streamableServer)
	httpMux.HandleFunc("/", rootRedirectHandler) // ✅ auto-create session + redirect to login page
	httpMux.HandleFunc("/mockWebPage", webPageHandler)
	httpMux.HandleFunc("/login", loginHandler)
	httpMux.HandleFunc("/generateSession", generateSessionHandler) // optional API

	port := pkg.GetPort()
	log.Println("starting server on port:", port)
	if servErr := http.ListenAndServe(fmt.Sprintf(":%s", port), httpMux); servErr != nil {
		log.Fatalln("error starting server", servErr)
	}
}

func dummyHandler(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("dummy handler"), nil
}

func webPageHandler(w http.ResponseWriter, r *http.Request) {
	sessionId := r.URL.Query().Get("sessionId")
	if sessionId == "" {
		http.Error(w, "sessionId is required", http.StatusBadRequest)
		return
	}

	tmpl, err := template.ParseFiles("static/login.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		SessionId            string
		AllowedMobileNumbers []string
	}{
		SessionId:            sessionId,
		AllowedMobileNumbers: pkg.GetAllowedMobileNumbers(),
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionId := r.FormValue("sessionId")
	phoneNumber := r.FormValue("phoneNumber")

	if sessionId == "" || phoneNumber == "" {
		http.Error(w, "sessionId and phoneNumber are required", http.StatusBadRequest)
		return
	}

	authMiddleware.AddSession(sessionId, phoneNumber)

	// ✅ Redirect to app UI
	http.Redirect(w, r, fmt.Sprintf("http://localhost:8080?sessionId=%s", sessionId), http.StatusFound)
}

// ✅ rootRedirectHandler generates session and redirects to login
func rootRedirectHandler(w http.ResponseWriter, r *http.Request) {
	sessionId, loginURL := authMiddleware.GenerateSessionAndLoginURL()
	log.Printf("New session started: %s\n", sessionId)
	http.Redirect(w, r, loginURL, http.StatusFound)
}

// ✅ Optional API to generate session programmatically
func generateSessionHandler(w http.ResponseWriter, r *http.Request) {
	sessionId, loginURL := authMiddleware.GenerateSessionAndLoginURL()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"sessionId":"%s","loginUrl":"%s"}`, sessionId, loginURL)
}