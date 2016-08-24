# gorfb

Remote Frame Buffer implementation in Golang according to the [RFC 6143 specification](http://www.rfc-base.org/txt/rfc-6143.txt). Please note that this is a work in progress and something I play with at the moment, it is not used in any production implementations (yet). At the moment the server side is provided with a client side being contemplated.

For an example on how to use this please look at [hduplooy/gorfb-conway](https://github.com/hduplooy/gorfb-conway).

### Current Issues

The raw format is obviously a bit slow when working over the internet. So the next step is implementing one of the encodings used by the protocol. I'll probably first look at TRLE and then ZRLE.



