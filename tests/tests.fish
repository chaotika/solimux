#!/usr/bin/fish

if not go build ..
  echo failed: build error
  exit 3
end


function my_signal_handler --on-signal USR1
  echo testcase \#$testn: $testcase
  echo aborted: took to long
    exit 2
end

set -g PID %self

function clean
  if test -d $tmp
    rm -fd $expected/* $actual/* $tmp/*
    rmdir $tmp
  end
end

function testcase
  set -g testn $argv[1]
  set -g testcase $argv[2..-1]
  #echo testcase \#$testn: $testcase
  set -g tmp tmp/$testn
  set -g expected $tmp/expected
  set -g actual $tmp/actual
  clean
  mkdir -p $tmp $expected $actual
  sh -c "sleep 5; kill -USR1 $PID; exec kill `ps -o pid= --ppid $PID | grep -v \$\$`;" &
end

function killbg
  if jobs -p 2> /dev/null 1> /dev/null
    kill (jobs -p)
  end
end

function check
  killbg
  set expectedsha (sh -c "cd $expected; sha256sum *")
  set actualsha (sh -c "cd $actual; sha256sum *")
  if [ "$expectedsha" != "$actualsha" ]
    echo testcase \#$testn: $testcase
    echo failed: has unexpected output
    #diff $expected $actual
    #bash -c "diff --side-by-side --suppress-common-lines <(xxd $expected) <(xxd $actual)"
    #cmp -b -l -n 64 $expected $actual
    exit 1
  end
  clean
end

testcase 1 word stdin
echo test | tee $expected/out \
  | ./solimux -i -o -echo > $actual/out
check

testcase 2 multiline random base64 stdin with a small linebuf
dd if=/dev/urandom bs=1k count=1 status=none | base64 | tee $expected/out \
  | ./solimux -i -o -echo -linebuf 77 > $actual/out
check

testcase 3 multiline random stdin
dd if=/dev/urandom bs=1k count=1 status=none | sed 's/\r\$//' > $expected/out
echo > $expected/out
cat $expected/out | ./solimux -i -o -echo > $actual/out
check

testcase 4 random JSON lines input
dd if=/dev/urandom bs=1k count=1024 status=none | jq -Rc | tee $expected/out \
  | ./solimux -i -o -echo -json > $actual/out
check

testcase 5 handle bad json
dd if=/dev/urandom bs=1k count=1024 status=none | jq -Rc > $expected/out
echo error LineReader: invalid JSON > $expected/err
mkfifo $tmp/stderr.fifo
cat $tmp/stderr.fifo | sed -E 's/[0-9]{4}\/[0-9]{2}\/[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2} //' > $actual/err &
begin echo bad-JSON-line; cat $expected/out; end | ./solimux -i -o -echo -json > $actual/out 2> $tmp/stderr.fifo
sleep 0.1
check

testcase 6 deal with one very long line
dd if=/dev/urandom bs=1k count=1024 status=none | jo line=%- | tee $expected/out \
  | ./solimux -i -o -echo -json -linebuf 2097152 > $actual/out
check

testcase 7 send some lines through a pipe with socat unidirectional with a small linebuf
./solimux -linebuf 77 $tmp/sock &
sleep 0.1
socat -u UNIX-CONNECT:$tmp/sock CREATE:$actual/lines &
cat /dev/urandom | base64 | head -n 1024 | tee $expected/lines | socat -u STDIN UNIX-CONNECT:$tmp/sock
check

testcase 8 send some lines through a pipe with socat bidirectional with a small linebuf
cat /dev/urandom | base64 | head -n 1024 > $expected/b.out
./solimux -linebuf 77 $tmp/sock &
sleep 0.1
cat /dev/urandom | base64 | head -n 1024 | tee $expected/b.out | sh -c "sleep 0.1; cat" | socat UNIX-CONNECT:$tmp/sock STDIO > $actual/a.out &
cat /dev/urandom | base64 | head -n 1024 | tee $expected/a.out | sh -c "sleep 0.1; cat" | socat UNIX-CONNECT:$tmp/sock STDIO > $actual/b.out &
sleep 0.2
kill %2
sleep 0.3
check

testcase 9 file reader
cat /dev/urandom | base64 | head -n 1024 > $expected/out
./solimux -file $expected/out $tmp/sock &
sleep 0.1
socat -u UNIX-CONNECT:$tmp/sock STDOUT > $actual/out &
sleep 0.1
kill %2
sleep 0.1
check

testcase 10 echo socket
cat /dev/urandom | base64 | head -n 1024 > $expected/out
./solimux -echo $tmp/sock &
sleep 0.1
socat UNIX-CONNECT:$tmp/sock STDIO < $expected/out > $actual/out &
sleep 0.1
kill %2
sleep 0.1
check

testcase 11 use file reader with echo
cat /dev/urandom | base64 | head -n 1 > $tmp/file
cat /dev/urandom | base64 | head -n 1 > $tmp/in
cat $tmp/file $tmp/in > $expected/out
./solimux -file $tmp/file -echo $tmp/sock &
sleep 0.1
socat UNIX-CONNECT:$tmp/sock STDIO < $tmp/in > $actual/out &
sleep 0.1
kill %2
sleep 0.1
check

#set fish_trace 1
