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

	// Start http server
	go runHttpServer(listen, &server, wg)

	// Wait for both http server and logger to shut down
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

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server listen failed: %+v\n", err)
		}
	}()
	log.Printf("Server listening on %v\n", listen)

	// Wait for termination signal
	<-done

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		log.Println("Server shutting down")
		waitDone := ctx.Done()
		cancel()

		// Wait for shutdown to complete then signal to main routine that we have completed
		<-waitDone
		wg.Done()
	}()

	// Start the shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed: %+v\n", err)
	}
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
	// Compact json element, so we are guaranteed it only uses a single line
	err = json.Compact(buf, data)
	if err != nil {
		// We probably received malformed JSON or something that is not JSON
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// Add newline to separate logged JSON elements
	_, _ = buf.Write([]byte{'\n'})

	// Message chan is unbuffered, so we will not complete the response until it have been received by the writer
	s.messageChan <- buf.Bytes()

	w.WriteHeader(http.StatusOK)
}
