(function () {
    'use strict';

    var methods = ['get', 'post', 'put', 'patch', 'delete', 'options', 'head'];
    var status = document.getElementById('api-docs-status');
    var endpoints = document.getElementById('api-docs-endpoints');

    function textElement(tag, className, value) {
        var element = document.createElement(tag);
        element.className = className;
        element.textContent = value;
        return element;
    }

    function operationsFrom(spec) {
        var operations = [];
        Object.keys(spec.paths || {}).sort().forEach(function (path) {
            methods.forEach(function (method) {
                var operation = spec.paths[path] && spec.paths[path][method];
                if (operation) {
                    operations.push({ method: method, path: path, operation: operation });
                }
            });
        });
        return operations;
    }

    function renderOperation(item) {
        var row = document.createElement('article');
        row.className = 'api-operation';
        row.dataset.method = item.method;
        row.appendChild(textElement('span', 'api-operation-method', item.method));
        row.appendChild(textElement('code', 'api-operation-path', item.path));
        row.appendChild(textElement('span', 'api-operation-summary', item.operation.summary || 'No summary provided'));

        var auth = Array.isArray(item.operation.security) && item.operation.security.length > 0;
        row.appendChild(textElement('span', 'api-operation-auth', auth ? 'Authentication' : 'Public'));
        return row;
    }

    function render(spec) {
        var operations = operationsFrom(spec);
        var info = spec.info || {};
        document.getElementById('api-docs-title').textContent = info.title || 'Platform API';
        document.getElementById('api-docs-description').textContent = info.description || 'Current HTTP API contract.';
        document.getElementById('api-docs-version').textContent = info.version || '—';
        document.getElementById('api-docs-path-count').textContent = String(Object.keys(spec.paths || {}).length);
        document.getElementById('api-docs-operation-count').textContent = String(operations.length);

        var fragment = document.createDocumentFragment();
        operations.forEach(function (operation) {
            fragment.appendChild(renderOperation(operation));
        });
        endpoints.replaceChildren(fragment);
        status.textContent = operations.length + ' operations loaded from /openapi.json';
    }

    function renderError(error) {
        status.textContent = 'The API contract could not be loaded.';
        var message = textElement('p', 'api-docs-error', 'OpenAPI loading failed. Use the raw JSON link or check server readiness.');
        endpoints.replaceChildren(message);
        if (window.console) {
            window.console.error('OpenAPI loading failed', error);
        }
    }

    fetch('/openapi.json', { headers: { Accept: 'application/json' } })
        .then(function (response) {
            if (!response.ok) {
                throw new Error('OpenAPI returned HTTP ' + response.status);
            }
            return response.json();
        })
        .then(render)
        .catch(renderError);
}());
