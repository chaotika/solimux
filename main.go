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
  "container/list"
)

const (
	MaxScanTokenSize = 1024 * 1024
)
var EchoLines bool
var StdIn bool
var StdOut bool
var LineBufferSize int
var AssumeJson bool
var RunForever bool
var ReadoutFileOnConnect string

var connectionsList *list.List

func main() {
  flag.BoolVar(&EchoLines, "echo", false, "echo lines back to sender")
  flag.BoolVar(&StdIn, "i", false, "read from stdin")
  flag.BoolVar(&StdOut, "o", false, "output all lines on stdout")
  flag.IntVar(&LineBufferSize, "linebuf", MaxScanTokenSize, "line buffer size in bytes")
  flag.BoolVar(&AssumeJson, "json", false, "verify every line to be valid JSON")
  flag.BoolVar(&RunForever, "forever", false, "do not end program when stdin is closed")
  flag.StringVar(&ReadoutFileOnConnect, "file", "", "read out file on connection")
  flag.Parse()

  connectionsList = list.New()

  var stdioConnection *Connection
  if StdOut && StdIn {
    stdioConnection = ReadWriteConnection(os.Stdin,os.Stdout)
  } else if !StdOut && StdIn {
    stdioConnection = ReadConnection(os.Stdin)
  } else if StdOut && !StdIn {
    stdioConnection = WriteConnection(os.Stdout)
  }

  for _, SocketPath := range flag.Args() {
    go NetListenServer("unix",SocketPath)
  }

  if StdIn && !RunForever {
    stdioConnection.wg.Wait()
  } else {
    select{ } // wait forever
  }

}

func NetListenServer(network, address string){
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
    ReadWriteConnection(conn,conn)
    //log.Printf("Client #%d connected [%s,%s]", connection.connectionId, conn.RemoteAddr().Network(), conn.RemoteAddr().String())
  }
}

type Connection struct {
  id int64
  reader io.Reader
  readable bool
	writer bufio.Writer
  writechan chan *[]byte
  writable bool
  wg sync.WaitGroup
  listElement *list.Element
}

func ReadConnection(reader io.Reader)(connection *Connection) {
  connection = &Connection{reader:reader,readable:true}
  connection.listElement = connectionsList.PushBack(connection)
  connection.wg.Add(1)
  go connection.LineReader()
  return connection
}

func WriteConnection(writer io.Writer)(connection *Connection) {
  connection = &Connection{writer:*bufio.NewWriterSize(writer,LineBufferSize+1),writable:true,writechan: make(chan *[]byte)}
  connection.listElement = connectionsList.PushBack(connection)
  if ReadoutFileOnConnect != "" {
    connection.ReadoutFile(ReadoutFileOnConnect)
  }
  go connection.LineWriter()
  return connection
}

func ReadWriteConnection(reader io.Reader, writer io.Writer)(connection *Connection) {
  connection = &Connection{reader:reader,readable:true,writer:*bufio.NewWriterSize(writer,LineBufferSize+1),writable:true,writechan: make(chan *[]byte)}
  connection.listElement = connectionsList.PushBack(connection)
  connection.wg.Add(2)
  go connection.LineReader()
  if ReadoutFileOnConnect != "" {
    connection.ReadoutFile(ReadoutFileOnConnect)
  }
  go connection.LineWriter()
  connection.wg.Done()
  return connection
}

func (connection *Connection) LineReader() {
  connection.LineScanner(connection.reader,
    func (line *[]byte){
      connection.wg.Add(1)
      connection.ConnectionsWriteLine(line)
      connection.wg.Done()
    },
    func (error string){
      log.Printf("error LineReader: %s",error)
    })
  connection.wg.Done()
  connection.Clean()
}

func (connection *Connection) ReadoutFile(filename string) {
  connection.wg.Add(1)
  file, err := os.Open(filename)
  if err != nil {
    log.Printf("ReadoutFile %s error %s",filename,err)
    connection.wg.Done()
    return
  }
  connection.LineScanner(file,
    func (line *[]byte){
      connection.WriteLineRaw(line)
    },
    func (error string){
      log.Printf("ReadoutFile %s invalid JSON",filename)
    })
  connection.wg.Done()
}

func (connection *Connection) LineScanner(reader io.Reader, successCb func(line *[]byte), errorCb func(error string) ){
  lineScanner := bufio.NewScanner(reader)
  lineScanner.Split(bufio.ScanLines)
  buf := make([]byte, LineBufferSize)
  lineScanner.Buffer(buf, LineBufferSize)
  for lineScanner.Scan() {
    line := lineScanner.Bytes()
    //line = append(line,"\n"...)
    if !AssumeJson || json.Valid(line) {
      successCb(&line)
    } else {
      errorCb("invalid JSON",)
    }
  }
}

func (connection *Connection) LineWriter() {
  for {
    line := <-connection.writechan
    connection.WriteLineRaw(line)
    connection.wg.Done()
  }
}

func (connection *Connection) WriteLine(line *[]byte) {
  if connection.writable {
    connection.wg.Add(1)
    connection.writechan <- line
  }
}

func (connection *Connection) WriteLineRaw(line *[]byte) {
  if connection.writable {
    _, err := connection.writer.Write(*line)
    if err != nil {
      log.Printf("write line error:", err)
      connection.writable = false
      defer connection.Clean()
      return
    }
    _, err = connection.writer.WriteString("\n")
    if err != nil {
      log.Printf("write newline error:", err)
      connection.writable = false
      defer connection.Clean()
      return
    }
    err = connection.writer.Flush()
    if err != nil {
      log.Printf("flush error:", err)
      connection.writable = false
      defer connection.Clean()
      return
    }
  }
}


func (sourceConnection *Connection) ConnectionsWriteLine(line *[]byte) {
  for e := connectionsList.Front(); e != nil; e = e.Next() {
    connection := e.Value.(*Connection)
    if connection != sourceConnection || EchoLines {
      connection.WriteLine(line)
    }
  }
}

func (connection *Connection) Clean() {
  //log.Println("Clean",connection.readable, connection.writable)
  if !connection.readable && !connection.writable {
    connectionsList.Remove(connection.listElement)
  }
}
