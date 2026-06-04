package clientrt

// Filename is the conventional output name for the generated client runtime.
const Filename = "gowdk.js"

// Source returns the first partial-update client runtime.
func Source() []byte {
	return []byte(runtimeSource)
}

const runtimeSource = `(function () {
  async function submitPartial(event) {
    var form = event.target && event.target.closest && event.target.closest('form[data-gowdk-target]');
    if (!form) {
      return;
    }

    var target = document.querySelector(form.dataset.gowdkTarget);
    if (!target) {
      return;
    }

    var before = new CustomEvent('gowdk:before-request', {
      cancelable: true,
      detail: { form: form, target: target }
    });
    if (!form.dispatchEvent(before)) {
      event.preventDefault();
      return;
    }

    event.preventDefault();
    form.setAttribute('aria-busy', 'true');
    var focused = focusTarget(document.activeElement);
    try {
      var response = await fetch(form.action, {
        method: (form.method || 'POST').toUpperCase(),
        body: new FormData(form),
        headers: {
          'X-GOWDK-Partial': '1',
          'X-GOWDK-Target': form.dataset.gowdkTarget,
          'X-GOWDK-Swap': form.dataset.gowdkSwap || ''
        }
      });
      if (!response.ok) {
        throw new Error('partial request failed with status ' + response.status);
      }
      var html = await response.text();
      var swap = response.headers.get('X-GOWDK-Fragment-Swap') || form.dataset.gowdkSwap || 'innerHTML';
      if (swap === 'outerHTML') {
        target.outerHTML = html;
      } else {
        target.innerHTML = html;
      }
      restoreFocus(focused);
      form.dispatchEvent(new CustomEvent('gowdk:after-swap', {
        detail: { form: form, target: target, swap: swap }
      }));
    } catch (error) {
      form.dispatchEvent(new CustomEvent('gowdk:request-error', {
        detail: { form: form, target: target, error: error }
      }));
    } finally {
      form.removeAttribute('aria-busy');
    }
  }

  document.addEventListener('submit', submitPartial);

  function focusTarget(element) {
    if (!element || element === document.body) {
      return {};
    }
    if (element.id) {
      return { id: element.id };
    }
    if (element.name) {
      return { name: element.name };
    }
    return {};
  }

  function restoreFocus(target) {
    if (!target) {
      return;
    }
    var element = target.id
      ? document.getElementById(target.id)
      : target.name
        ? document.querySelector('[name="' + escapeAttr(target.name) + '"]')
        : null;
    if (element && element.focus) {
      element.focus();
    }
  }

  function escapeAttr(value) {
    return String(value).replace(/["\\]/g, '\\$&');
  }
}());
`
