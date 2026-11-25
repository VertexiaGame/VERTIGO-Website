export function stripOBJLines(text) {
    return String(text).replace(/^[ \t]*[lp][ \t][^\r\n]*\r?\n?/gm, '');
}

export function parseSolidOBJ(OBJLoader, text) {
    const model = new OBJLoader().parse(stripOBJLines(text));
    const stray = [];
    model.traverse(child => {
        if (child !== model && !child.isMesh && (child.isLine || child.isPoints)) stray.push(child);
    });
    for (const child of stray) child.removeFromParent();
    return model;
}

export async function loadSolidOBJ(OBJLoader, url, init) {
    const res = await fetch(url, init);
    if (!res.ok) throw new Error('Failed to load ' + url + ' (' + res.status + ')');
    return parseSolidOBJ(OBJLoader, await res.text());
}
