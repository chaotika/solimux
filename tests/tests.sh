#!/usr/bin/fish

go build ..

function clean
  rm $actual $expected
end

function setup
set testn $argv[1]
  echo test \#$testn $argv[2..-1]
  set uuid (uuid -v4)
  set -g expected tmp/$testn-expected-$uuid
  set -g actual tmp/$testn-actual-$uuid
end

function killbg
  if jobs -p 2> /dev/null 1> /dev/null
    kill (jobs -p)
  end
end

function check
  set expectedsha (sha256sum < $expected)
  set actualsha (sha256sum < $actual)
  if [ "$expectedsha" = "$actualsha" ]
    echo passed
  else
    echo failed
    #diff $expected $actual
    #bash -c "diff --side-by-side --suppress-common-lines <(xxd $expected) <(xxd $actual)"
    cmp -b -l -n 64 $expected $actual
    exit 1
  end
  clean
  echo
end

setup 1 word stdin
echo test | tee $expected \
  | ./solimux -i -o -echo > $actual
check

setup 2 multiline random base64 stdin
dd if=/dev/urandom bs=1k count=1 status=none | base64 | tee $expected \
   | ./solimux -i -o -echo > $actual
check

setup 3 multiline random stdin
dd if=/dev/urandom bs=1k count=1 status=none | sed 's/\r\$//' > $expected
echo > $expected
cat $expected | ./solimux -i -o -echo > $actual
check

setup 4 random JSON lines input
dd if=/dev/urandom bs=1k count=1024 status=none | jq -Rc | tee $expected \
  | ./solimux -i -o -echo -json > $actual
check

setup 5 handle bad json
dd if=/dev/urandom bs=1k count=1024 status=none | jq -Rc > $expected
begin echo bad-JSON-line; cat $expected; end | ./solimux -i -o -echo -json > $actual
check

setup 6 deal with one very long line
dd if=/dev/urandom bs=1k count=1024 status=none | jo line=%- | tee $expected | ./solimux -i -o -echo -json -linebuf 2097152 > $actual
check

setup 7 send some lines through a pipe with socat
if ! test -p
  mkfifo tmp/7-input.fifo
end
./solimux tmp/7.sock &
socat -u UNIX-CONNECT:tmp/7.sock CREATE:$actual &
cat /dev/urandom | base64 | head -n 1024 | tee $expected | socat -u STDIN UNIX-CONNECT:tmp/7.sock
killbg
check
