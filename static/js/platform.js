document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('.toast').forEach(function (toast) {
        setTimeout(function () { toast.remove(); }, 5000);
    });

    var themeToggle = document.getElementById('theme-toggle');
    if (themeToggle) {
        themeToggle.addEventListener('click', function () {
            var html = document.documentElement;
            var current = html.getAttribute('data-theme');
            var next = current === 'dark' ? 'light' : 'dark';
            html.setAttribute('data-theme', next);
            fetch('/settings/preferences', {
                method: 'POST',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: 'theme=' + next
            }).then(function (response) {
                if (!response.ok) throw new Error('Unable to save theme');
            }).catch(function () {
                html.setAttribute('data-theme', current);
                window.alert('The theme could not be saved. Please try again.');
            });
        });
    }

    var csrfMeta = document.querySelector('meta[name="csrf-token"]');
    if (csrfMeta) {
        var csrfToken = csrfMeta.getAttribute('content');
        var originalFetch = window.fetch;
        window.fetch = function (url, options) {
            options = options || {};
            options.headers = options.headers || {};
            if (options.method && options.method !== 'GET') {
                options.headers['X-CSRF-Token'] = csrfToken;
            }
            return originalFetch(url, options);
        };
    }

    document.querySelectorAll('[data-confirm]').forEach(function (el) {
        el.addEventListener('click', function (e) {
            if (!confirm(el.getAttribute('data-confirm'))) {
                e.preventDefault();
            }
        });
    });
});
