# Question

Design the complete HTTP caching strategy for these three endpoints:
```
GET /articles/456        # article content, changes rarely
GET /authors/789         # author bio, changes very rarely
GET /articles/456/comments  # comments, change frequently  
```
---

# Answer

## 1. GET /articles/456        # article content, changes rarely

```http
GET /articles/456
Cache-Control: public, max-age=86400, stale-while-revalidate=3600, stale-if-error=86400
ETag: "art456-hash123"
Last-Modified: Wed, 10 Jun 2026 08:00:00 GMT
Vary: Accept-Encoding
Cache-Tag: article-456, author-789
```

## 2. GET /authors/789         # author bio, changes very rarely

```http
GET /authors/789
Cache-Control: public, max-age=86400, stale-while-revalidate=3600, stale-if-error=86400
ETag: "auth789-hash456"
Last-Modified: Wed, 10 Jun 2026 08:00:00 GMT
Vary: Accept-Encoding
Cache-Tag: author-789
```

## 3. GET /articles/456/comments  # comments, change frequently  

```http
GET /articles/456/comments
Cache-Control: public, max-age=30, stale-while-revalidate=10, stale-if-error=3600
ETag: "comm456-hash789"
Last-Modified: Wed, 10 Jun 2026 08:00:00 GMT
Vary: Accept-Encoding
Cache-Tag: comments-456, article-456
```