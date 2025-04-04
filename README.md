# GO.BLOK DoH (DNS-over-HTTPS)

## Description  
This is a DNS-over-HTTPS (DoH) server implementation that supports caching, rate limiting, and DNS query logging. The server receives DNS queries from clients over UDP, checks the cache, and forwards queries to a DoH resolver if the data is not available in the cache.  

FYI, you probably don’t need this code. I built it to suit my own needs, so if you don’t need it, just leave this GitHub page.  
If you decide to use this source code, any difficulties are **your own responsibility**. Don’t even think about asking me for help or support.  

Learn it yourself!  

**Note:**  
The name `GO.BLOK` is a play on words: **GOBLOK** = **BODoH** = **STUPID**  

## Features  
- Supports DNS queries over UDP (like a typical DNS server)  
- Uses a DoH resolver as an upstream  
- Round-robin upstream selection  
- Caching for better performance  
- IP-based rate limiting to prevent abuse  
- DNS query logging for analysis  

## Upcoming Features  
- Domain-based blocking list  
- Frontend: Monitoring dashboard  
- Frontend: Upstream management, blocking lists, logs, etc.  
- ... (I'll think about more later)  

## Installation & Configuration  
### **Choose and Download the release**  
release page [`https://github.com/sudo-goblok/go.blok.doh/releases`](https://github.com/sudo-goblok/go.blok.doh/releases)


### Docker or Podman  
#### Build Image  
```sh
docker compose build --no-cache
```
#### Run Container  
```sh
docker compose up
```
#### Configuration  
The `config` directory in `docker-compose.yml` is mounted as a host volume.  

Unless you are running without a container, the `config.yaml` file inside it does not need to be edited. If necessary, simply comment/uncomment the existing resolver entries. That’s it! 

The `config.yaml` file allows you to configure the DoH resolver, server port, and other parameters.  

### Example Query Using `dig`  
Adjust it to your environment:  
```sh
dig @127.0.0.1 -p 53 yahoo.com A
```

## License  
MIT License
