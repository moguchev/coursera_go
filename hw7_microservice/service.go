package main

import (
	context "context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	sync "sync"
	"time"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные

type ACL map[string][]string

type statistic map[string]uint64

type Server struct {
	acl ACL
	ctx context.Context

	logChan  chan *Event
	statChan chan *Stat

	sync.Mutex
	connLog  []chan *Event
	connStat []chan *Stat
}

func StartMyMicroservice(ctx context.Context, addr, acl string) error {
	s := &Server{ctx: ctx}
	// acl - json string
	err := json.Unmarshal([]byte(acl), &s.acl)
	if err != nil {
		log.Print(err)
		return err
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Print("cant listen port", err)
		return err
	}

	server := grpc.NewServer(
		// перехватчик стримов
		grpc.StreamInterceptor(s.streamInterceptor),
		// перехватчик одиночных запросов
		grpc.UnaryInterceptor(s.countInterceptor),
	)
	RegisterBizServer(server, s)
	RegisterAdminServer(server, s)

	// запуск сервиса по адресу
	go func() {
		fmt.Printf("starting server at %s\n", addr)
		err = server.Serve(lis)
		if err != nil {
			log.Fatal(err)
		}
	}()
	// остановка сервиса
	go func() {
		<-ctx.Done()
		server.Stop()
		fmt.Printf("stop server at %s\n", addr)
	}()

	s.logChan = make(chan *Event, 0)
	s.statChan = make(chan *Stat, 0)
	// лог
	go s.ListenerLog()

	return nil
}

// запись событий(логов) во все каналы
func (s *Server) ListenerLog() {
	for {
		select {
		case stat := <-s.statChan:
			s.Lock()
			for _, channel := range s.connStat {
				channel <- stat
			}
			s.Unlock()
		case event := <-s.logChan:
			s.Lock()
			for _, channel := range s.connLog {
				channel <- event
			}
			s.Unlock()
		case <-s.ctx.Done():
			return
		}
	}
}

// контроль доступа от разных клиентов
func (s *Server) checkAccess(ctx context.Context, method string) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", grpc.Errorf(codes.Unauthenticated, "error get metadata")
	}

	consumer, ok := md["consumer"]
	if !ok {
		return "", grpc.Errorf(codes.Unauthenticated, "error get consumer")
	}
	// should only have 1 consumer
	if len(consumer) != 1 {
		grpc.Errorf(codes.Unauthenticated, "can't get metadata")
	}

	validPaths, ok := s.acl[consumer[0]]
	if !ok {
		return "", grpc.Errorf(codes.Unauthenticated, "error get acl")
	}

	var allowedCall bool
	if strings.Contains(validPaths[0], "*") {
		if strings.Split(validPaths[0], "/")[0] == strings.Split(method, "/")[0] {
			allowedCall = true
		}
	} else {
		for _, p := range validPaths {
			if p == method {
				allowedCall = true
			}
		}
	}
	if !allowedCall {
		return "", grpc.Errorf(codes.Unauthenticated, "access error")
	}

	return consumer[0], nil
}

func (s *Server) countInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	consumer, err := s.checkAccess(ctx, info.FullMethod)
	if err != nil {
		return nil, err
	}

	event := Event{
		Consumer: consumer,
		Method:   info.FullMethod,
		Host:     "127.0.0.1:8083",
	}

	stat := Stat{
		ByConsumer: statistic{consumer: +1},
		ByMethod:   statistic{info.FullMethod: +1},
	}

	s.logChan <- &event
	s.statChan <- &stat

	return handler(ctx, req)
}

func (s *Server) streamInterceptor(
	srv interface{},
	serverStream grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	consumer, err := s.checkAccess(serverStream.Context(), info.FullMethod)
	if err != nil {
		return err
	}

	var event = Event{
		Consumer: consumer,
		Method:   info.FullMethod,
		Host:     "127.0.0.1:8083",
	}

	var stat = Stat{
		ByConsumer: statistic{consumer: +1},
		ByMethod:   statistic{info.FullMethod: +1},
	}

	s.logChan <- &event
	s.statChan <- &stat

	return handler(srv, serverStream)
}

// Бизнес логика

func (s *Server) Check(ctx context.Context, n *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

func (s *Server) Add(ctx context.Context, n *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

func (s *Server) Test(ctx context.Context, n *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

// Модуль администрирования

func (s *Server) Logging(nothing *Nothing, srv Admin_LoggingServer) error {
	ch := make(chan *Event, 0)
	s.Lock()
	// добавляем канал(клиента) логгирования
	s.connLog = append(s.connLog, ch)
	s.Unlock()

	for {
		select {
		case event := <-ch:
			// отправка события
			err := srv.Send(event)
			if err != nil {
				return err
			}
		case <-s.ctx.Done():
			// завершаем работу
			return nil
		}
	}
}

func (s *Server) Statistics(interval *StatInterval, srv Admin_StatisticsServer) error {
	ch := make(chan *Stat, 0)
	s.Lock()
	// добавляем канал(клиента) статистики
	s.connStat = append(s.connStat, ch)
	s.Unlock()

	ticker := time.NewTicker(time.Second * time.Duration(interval.IntervalSeconds))
	var res = &Stat{
		ByMethod:   make(statistic),
		ByConsumer: make(statistic),
	}
	for {
		select {
		case stat := <-ch:
			// подсчёт статистики
			for k, v := range stat.ByMethod {
				res.ByMethod[k] += v
			}
			for k, v := range stat.ByConsumer {
				res.ByConsumer[k] += v
			}
		case <-ticker.C:
			// отправка статистики по истечении таймера
			err := srv.Send(res)
			if err != nil {
				return err
			}
			res = &Stat{
				ByMethod:   make(statistic),
				ByConsumer: make(statistic),
			}
		case <-s.ctx.Done():
			// завершаем работу
			return nil
		}
	}
}
