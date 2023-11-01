package files

import (
	"context"

	"github.com/thejerf/suture/v4"
)

type Server struct {
	config *Config
}

func NewServer(config *Config) *Server {
	return &Server{
		config: config,
	}
}

func (s *Server) Run() error {
	supervisor := suture.NewSimple("server")

	err := OpenDatabase("files.db")
	if err != nil {
		panic(err)
	}

	fileStore := NewFileStore(s.config)

	if s.config.HTTP != nil {
		httpService := NewHTTPService(s.config, fileStore)
		supervisor.Add(httpService)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return supervisor.Serve(ctx)
}
