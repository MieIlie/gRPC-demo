export async function loadView(name) {
    const res = await fetch(`/components/${name}.html`);
    if (!res.ok) {
        throw new Error(`Failed to load view ${name}`);
    }
    const html = await res.text();
    const appEl = document.getElementById('app');
    appEl.innerHTML = html;
}

export function showLoading(text = "Loading...") {
    const appEl = document.getElementById('app');
    appEl.innerHTML = `
        <div class="loader-container">
            <div class="spinner"></div>
            <p>${text}</p>
        </div>
    `;
}

export function getElement(id) {
    return document.getElementById(id);
}
