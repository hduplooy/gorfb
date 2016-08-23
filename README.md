# gorfb

Remote Frame Buffer implementation in Golang according to the [RFC 6143 specification](http://www.rfc-base.org/txt/rfc-6143.txt). Please note that this is a work in progress and something I play with at the moment, it is not used in any production implementations (yet). At the moment the server side is provided with a client side being contemplated.

For an example on how to use this please look at [hduplooy/gorfb-examples](https://github.com/hduplooy/gorfb-examples).

## Current Issues

The library works fine for most VNC clients, but some have a problem with bytes that are out of place. Just need to make provision for it.


