/* Swagger UI 5.34.1 is vendored alongside this file; no CDN or remote validator. */
window.onload = function () {
  window.ui = SwaggerUIBundle({
    url: '/api/docs/openapi.json',
    dom_id: '#swagger-ui',
    deepLinking: true,
    displayRequestDuration: true,
    docExpansion: 'none',
    persistAuthorization: false,
    withCredentials: true,
    validatorUrl: null,
    requestInterceptor: function (request) {
      // The browser supplies Origin and HttpOnly cookies; never copy tokens to storage.
      if (request.loadSpec) return request;
      request.headers.Accept = 'application/json';
      return request;
    }
  });
};
