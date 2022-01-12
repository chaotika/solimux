package main

import (
  "flag"
  "log"
  "sync"
  "encoding/json"
  "os"
  "net"
  "io"
  "bufio"
  "container/ring"
)

var EchoLines bool
var StdIn bool
var StdOut bool
var LineBufferSize int
var AssumeJson bool
var ReadoutFileOnConnect string

var connectionsStore ConnectionsStore

func main() {
  flag.BoolVar(&EchoLines, "echo", false, "echo lines back to sender")
  flag.BoolVar(&StdIn, "i", false, "read from stdin")
  flag.BoolVar(&StdOut, "o", false, "output all lines on stdout")
  flag.IntVar(&LineBufferSize, "linebuf", 128 * 1024 * 1024, "line buffer size in bytes")
  flag.BoolVar(&AssumeJson, "json", false, "verify every line to be valid JSON")
  flag.StringVar(&ReadoutFileOnConnect, "file", "", "read out file on connection")
  flag.Parse()
  connectionsStore = ConnectionsStore{connections: make(map[int64]Connection)}
  var stdioConnection *Connection
  if StdOut && StdIn {
    stdioConnection = connectionsStore.ReadWriteConnection(os.Stdin,os.Stdout)
  } else if !StdOut && StdIn {
    stdioConnection = connectionsStore.ReadConnection(os.Stdin)
  } else if StdOut && !StdIn {
    stdioConnection = connectionsStore.WriteConnection(os.Stdout)
  } else {
    connectionsStore.connectionIdCounter++
  }
  for _, SocketPath := range flag.Args() {
    go NetListenServer("unix",SocketPath,&connectionsStore)
  }
  if StdIn {
    stdioConnection.wg.Wait()
  } else {
    select{ } // wait forever
  }

}

func NetListenServer(network, address string, connectionsStore *ConnectionsStore){
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
    connectionsStore.ReadWriteConnection(conn,conn)
    //log.Printf("Client #%d connected [%s,%s]", connection.connectionId, conn.RemoteAddr().Network(), conn.RemoteAddr().String())
  }
}

func (connectionsStore *ConnectionsStore) ReadConnection(reader io.Reader)(connection *Connection) {
  connectionsStore.mutex.Lock()
  connection = connectionsStore.AddConnection(Connection{reader:reader})
  connection.wg.Add(1)
  go connection.LineReader()
  connectionsStore.mutex.Unlock()
  return connection
}
func (connectionsStore *ConnectionsStore) WriteConnection(writer io.Writer)(connection *Connection) {
  connectionsStore.mutex.Lock()
  connection = connectionsStore.AddConnection(Connection{writer:writer,writeTo:true})
  if ReadoutFileOnConnect != "" {
    connection.ReadoutFile(ReadoutFileOnConnect)
  }
  connectionsStore.mutex.Unlock()
  return connection
}
func (connectionsStore *ConnectionsStore) ReadWriteConnection(reader io.Reader, writer io.Writer)(connection *Connection) {
  connectionsStore.mutex.Lock()
  connection = connectionsStore.AddConnection(Connection{reader:reader,writer:writer,writeTo:false})
  connection.wg.Add(1)
  go connection.LineReader()
  if ReadoutFileOnConnect != "" {
    connection.ReadoutFile(ReadoutFileOnConnect)
  }
  connection.writeTo = true
  return connection
  connectionsStore.mutex.Unlock()
}


func (connectionsStore *ConnectionsStore) Write(data []byte, sourceConnection *Connection) {
  connectionsStore.mutex.Lock()
  for connectionId, connection := range connectionsStore.connections {
    if connection.writeTo && (connectionId != sourceConnection.connectionId || EchoLines)  {
      _, err := connection.writer.Write(data)
      if err != nil {
        log.Printf("write error on #%d:", connectionId, err)
        delete(connectionsStore.connections, connectionId);
      }
    }
  }
  connectionsStore.mutex.Unlock()
}

type Connection struct {
  connectionId int64
  reader io.Reader
	writer io.Writer
  writeTo bool
  wg sync.WaitGroup
}

func (connection *Connection) LineReader() {
  lineScanner := bufio.NewScanner(connection.reader)
  lineScanner.Split(bufio.ScanLines)
  buf := make([]byte, LineBufferSize)
  lineScanner.Buffer(buf, LineBufferSize)
  for lineScanner.Scan() {
    line := lineScanner.Bytes()
    if !AssumeJson || json.Valid(line) {
      connectionsStore.Write(append(line,"\n"...),connection)
    } else {
      log.Printf("invalid JSON",)
    }
  }
  connection.wg.Done()
}

func (connection *Connection) ReadoutFile(filename string) {
  file, err := os.Open(filename)
  if err != nil {
    log.Printf("ReadoutFile %s error %s",filename,err)
    return
  }
  lineScanner := bufio.NewScanner(file)
  lineScanner.Split(bufio.ScanLines)
  buf := make([]byte, LineBufferSize)
  lineScanner.Buffer(buf, LineBufferSize)
  for lineScanner.Scan() {
    line := lineScanner.Bytes()
    if !AssumeJson || json.Valid(line) {
      connection.writer.Write(append(line,"\n"...))
    } else {
      log.Printf("ReadoutFile %s invalid JSON",filename)
    }
  }
}

// func (connection *Connection) LineWriter() {
//   connection.writeTo = true
// }
