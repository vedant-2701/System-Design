# Question

> You're designing a public REST API for a food delivery platform. A junior engineer on your team says: "Let's just use POST for everything — it's simpler, we don't have to think about which verb to use, and POST works for all cases."
> Make the engineering case against this decision. Cover correctness, caching, idempotency, and client experience. Argue it like you're in a design review.

---

# Answer

**The case against POST-for-everything — argued precisely:**

**One — caching breaks completely.**

GET requests are cacheable by definition. Browsers, CDNs, and proxies know this and cache GET responses automatically based on Cache-Control headers.

POST is never cached. By HTTP specification, POST signals a state-changing operation. No CDN will cache a POST response.

Your food delivery platform has `GET /restaurants` — potentially millions of requests per hour for the same restaurant listings. With GET, one CDN edge node serves thousands of users from cache. With POST `/getRestaurants`, every single request hits your origin server. You've eliminated your entire caching layer and handed your database a direct 50k req/s problem.

This is not a convention violation. This is a performance catastrophe.

**Two — idempotency guarantees disappear.**

GET and PUT are idempotent by specification. This means HTTP clients, load balancers, and proxies know they can safely retry these requests on network failure.

POST is not idempotent. When a POST request times out, the client cannot know whether the server processed it or not. Did the order get created? Was the payment charged? Should I retry?

With POST-for-everything, your retry logic now lives entirely in application code for every single endpoint. You've lost the protocol-level safety net.

Consider your order creation flow — already POST, correctly non-idempotent. With idempotency keys, you handle this explicitly. But now imagine your `POST /getOrderStatus` times out. The client retries. Did it fetch status once or twice? Doesn't matter — but your load balancer doesn't know it's safe to retry, so it might not. You've created ambiguity where the protocol would have given you clarity for free.

**Three — client experience degrades immediately.**

Browser back button behavior, bookmarking, link sharing — all depend on GET semantics. A restaurant page at `/restaurants/123` can be bookmarked, shared, opened in a new tab, cached by the browser. 

`POST /getRestaurant` with body `{"id": 123}` cannot be bookmarked. Cannot be shared as a URL. Browser history breaks. Every client — browser, mobile app, third party integration — has to build custom handling for things the protocol gives you for free with GET.

**Four — operational visibility disappears.**

Every monitoring tool, every CDN analytics dashboard, every access log distinguishes GET from POST. You can immediately see read traffic vs write traffic, cache hit rates, which resources are being read most.

With POST-for-everything, your logs show 100% POST traffic. You cannot distinguish reads from writes without parsing request bodies. Debugging a production incident where reads are slow becomes significantly harder.

**Five — the simplicity argument is false.**

The junior engineer said POST is simpler. Simpler for whom? Simpler to write on day one. But every client consuming your API now has to read documentation to understand what each endpoint does — there's no way to infer it from the method. Every new engineer onboarding has to learn your custom conventions. Every integration partner writes more code to handle your non-standard API.

You've traded 10 minutes of simplicity for months of accumulated client complexity.

---

**What you should have said in your answer:**

Your answer listed solutions — use proper verbs, use caching, use OpenAPI. But the question asked you to argue *against* the junior engineer's proposal. That means attacking the proposal's consequences, not listing alternatives.

The structure of a good design review argument is:

One — here is what breaks concretely. Two — here is who it affects. Three — here is the scale of the damage. Four — here is the alternative and why it's better.

---