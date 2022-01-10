package main

import (
  "log"
  "sync"
  "encoding/json"
  "os"
  "net"
  "io"
  "bufio"
)

func main() {
  writerStore := WriterStore{writers: make(map[int64]io.Writer)}
  writerStore.AddWriter(os.Stdout)
  go NetListenServer("unix",os.Args[1],writerStore)
  LineReader(os.Stdin, writerStore)
}

const MaxScanTokenSize = 128 * 1024 * 1024
const AssumeJson = true

func LineReader(reader io.Reader, writerStore WriterStore) {
  lineScanner := bufio.NewScanner(reader)
  lineScanner.Split(bufio.ScanLines)
  buf := make([]byte, MaxScanTokenSize)
  lineScanner.Buffer(buf, MaxScanTokenSize)
  for lineScanner.Scan() {
    line := lineScanner.Bytes()
    if !AssumeJson || json.Valid(line) {
      writerStore.Write(append(line,"\n"...))
    } else {
      log.Printf("invalid JSON",)
    }
  }
}

type WriterStore struct {
	mutex sync.Mutex
  connectionIdCounter int64
	writers  map[int64]io.Writer
}

func (writerStore *WriterStore) AddWriter(writer io.Writer)(writerConnectionId int64) {
	writerStore.mutex.Lock()
  writerStore.connectionIdCounter++
  writerConnectionId = writerStore.connectionIdCounter
  writerStore.writers[writerConnectionId] = writer
	writerStore.mutex.Unlock()
  return writerConnectionId
}

func (writerStore *WriterStore) Write(data []byte) {
  writerStore.mutex.Lock()
  for writerConnectionId, writer := range writerStore.writers {
    _, err := writer.Write(data )
    if err != nil {
      log.Printf("write error on #%d:", writerConnectionId, err)
      delete(writerStore.writers, writerConnectionId);
    }
  }
  writerStore.mutex.Unlock()
}

func NetListenServer(network, address string, writerStore WriterStore){
  if err := os.RemoveAll(address); err != nil {
    log.Fatal(err)
  }

  listener, err := net.Listen(network, address)
  if err != nil {
    log.Fatal("listen error:", err)
  }
  defer listener.Close()


  for {
    conn, err := listener.Accept()
    if err != nil {
        log.Fatal("accept error:", err)
    }
    writerConnectionId := writerStore.AddWriter(conn)
    log.Printf("Client #%d connected [%s,%s]", writerConnectionId, conn.RemoteAddr().Network(), conn.RemoteAddr().String())
    go LineReader(conn,writerStore)

  }
}
