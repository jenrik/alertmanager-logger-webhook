package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	// Configuration
	listen, ok := os.LookupEnv("LOGGER_LISTEN")
	if !ok {
		listen = "0.0.0.0:8080"
	}
	output, ok := os.LookupEnv("LOGGER_OUTPUT")
	if !ok {
		output = "./logs/alerts.log"
	}

	// Coordinate server and logger shutdown
	wg := &sync.WaitGroup{}
	wg.Add(2)

	serverCtx, serverCancel := context.WithCancel(context.Background())
	server := LoggerServer{
		messageChan: make(chan []byte),
		reloadChan:  make(chan os.Signal),
		ctx:         serverCtx,
		wg:          wg,
		logger: lumberjack.Logger{
			Filename:   output,
			MaxBackups: 3,
			Compress:   false,
		},
	}
	signal.Notify(server.reloadChan, syscall.SIGHUP)

	go func() {
		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

		<-done

		serverCancel()
	}()

	// Start alert writer
	go server.writer()

	go runHttpServer(listen, &server, wg)

	wg.Wait()
	log.Println("Shutdown complete")
}

func runHttpServer(listen string, server *LoggerServer, wg *sync.WaitGroup) {
	// Setup alert receiver server
	router := mux.NewRouter()
	router.HandleFunc("/log", server.log)
	srv := &http.Server{
		Addr:    listen,
		Handler: router,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server listen failed: %+v\n", err)
		}
	}()
	log.Printf("Server listening on %v\n", listen)

	<-done

	defer func() {
		log.Println("Server shutting down")
		cancel()
	}()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed: %+v\n", err)
	}

	wg.Done()
}

type LoggerServer struct {
	messageChan chan []byte
	reloadChan  chan os.Signal
	ctx         context.Context
	wg          *sync.WaitGroup
	logger      lumberjack.Logger
}

func (s *LoggerServer) writer() {
	done := s.ctx.Done()
	for {
		select {
		case message := <-s.messageChan:
			_, _ = s.logger.Write(message)
		case <-s.reloadChan:
			_ = s.logger.Rotate()
		case <-done:
			_ = s.logger.Close()
			s.wg.Done()
			return
		}
	}
}

func (s *LoggerServer) log(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	buf := bytes.NewBuffer(nil)
	err = json.Compact(buf, data)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	_, _ = buf.Write([]byte{'\n'})
	s.messageChan <- buf.Bytes()

	w.WriteHeader(http.StatusOK)
}
