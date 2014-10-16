<?php

stream_set_blocking(STDIN, 0);
while(true) {
    $line = chop(fgets(STDIN));
    if($line=='STOP') {
        break;
    } else if($line=='SHRINK') {
        echo "SHRINKED\n";
        break;
    } else if($line=='RESTART') {
        echo "EXITED for restart\n";
        break;
    }
    echo time()."\n";
    sleep(rand(1,10));
}
