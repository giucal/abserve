# [abserve(1)]

[abserve(1)]: man.pod

Minimal HTTP server that serves a *virtual* resource directly from
memory.

    % echo '<p>Hello, world!</p>' | abserve /index.html &
    [1] 56382
    % curl http://localhost:8080/index.html
    <p>Hello, world!</p>

## Overview

The resource is read either once and for all from standard input or, with the
`--poll` option, repeatedly from a FIFO:

    % mkfifo resource.fifo
    % abserve --poll resource.fifo &
    [1] 56385
    % echo A > resource.fifo
    % curl http://localhost:8080
    A

Writing to the FIFO causes abserve to update the resource:

    % echo B > resource.fifo      # update
    % curl http://localhost:8080
    B
    % curl http://localhost:8080  # the resource is cached
    B

It's also possible to serve *concrete* resources from a given
directory. The virtual resource takes precedence over anything else.

<pre><code>% ls
cat.jpg   dog.jpg   index.html
% cat index.html                            # index.html on disk
&lt;img src="dog.jpg"&gt;
% echo '&lt;img src="cat.jpg"&gt;' | abserve --directory . /index.html &amp;
[1] 56388
% curl http://localhost:8080/index.html     # virtual index.html
&lt;img src="cat.jpg"&gt;
% curl http://localhost:8080/cat.jpg | catimg
<img alt="cat.jpg"
     src="https://gist.githubusercontent.com/giucal/282bf150c6001ae1028bcd92ac3f5f5c/raw/cat.jpg"
     title="Copyright 2006 Giuseppe Calabrese. All Rights Reserved."
     height=100>
</code></pre>

Enough with cats. For details, see the [man page][abserve(1)].

## Installation

    go install github.com/giucal/abserve@latest
