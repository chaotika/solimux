#!/usr/bin/fish

function solimux
  go run . $argv[1..-1]
end

function clean
  rm $actual $expected
end

function setup
  echo $argv
  set -g expected (mktemp)
  set -g actual (mktemp)
end

function check
  set expectedsha (sha256sum < $expected)
  set actualsha (sha256sum < $actual)
  if [ "$expectedsha" = "$actualsha" ]
    echo passed
  else
    echo failed
    echo
    echo diff
    echo
    diff $expected $actual
    echo
    echo hex diff
    echo
    bash -c "diff <(xxd $expected) <(xxd $actual)"
    clean
    exit 1
  end
  clean
  echo
end

setup test \#1 word stdin
echo test > $expected
cat $expected | solimux -i -o -echo > $actual
check

setup test \#2 multiline random base64 stdin
dd if=/dev/urandom bs=1k count=1 status=none | base64 > $expected
cat $expected | solimux -i -o -echo > $actual
check

setup test \#3 multiline random stdin
dd if=/dev/urandom bs=1k count=1 status=none > $expected
echo >> $expected
cat $expected | solimux -i -o -echo > $actual
check

setup test \#4 random JSON lines input
dd if=/dev/urandom bs=1k count=1024 status=none | jq -Rc > $expected
cat $expected | solimux -i -o -echo -json > $actual
check

setup test \#5 handle bad json
dd if=/dev/urandom bs=1k count=1024 status=none | jq -Rc > $expected
begin echo bad-JSON-line; cat $expected; end | solimux -i -o -echo -json > $actual
check

setup test \#6 deal with one very long line
dd if=/dev/urandom bs=1k count=1024 status=none | jo line=%- > $expected
cat $expected | solimux -i -o -echo -json -linebuf 2097152 > $actual
check
