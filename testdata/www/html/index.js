(() => {
    "use strict";

    const LS_KEY = "prestd_console_settings_v1";

    function loadSettings() {
        try { return JSON.parse(localStorage.getItem(LS_KEY) || "{}"); }
        catch (e) { return {}; }
    }

    function persistSettings(partial) {
        const current = loadSettings();
        localStorage.setItem(LS_KEY, JSON.stringify({ ...current, ...partial }));
    }

    function saveSettings() {
        const s = {
            baseUrl: $("#baseUrl").value.trim(),
            database: $("#cfgDatabase").value.trim(),
            schema: $("#cfgSchema").value.trim(),
            table: $("#cfgTable").value.trim(),
            authType: $("#authType").value,
            authToken: $("#authToken").value.trim(),
            authUser: $("#authUser").value.trim(),
            authPass: $("#authPass").value,
            sidebarWidth: $("#sidebar") ? $("#sidebar").offsetWidth : undefined,
        };
        persistSettings(s);
    }

    function $(sel, root = document) { return root.querySelector(sel); }
    function $all(sel, root = document) { return Array.from(root.querySelectorAll(sel)); }
    function el(tag, cls, html) {
        const element = document.createElement(tag);
        if (cls) {
            element.className = cls;
        }
        if (html !== undefined) {
            element.innerHTML = html;
        }
        return element;
    }

    function toast(msg) {
        $("#toastBody").textContent = msg;
        new bootstrap.Toast($("#toastEl"), { delay: 2600 }).show();
    }

    $all("#mainTabs .nav-link").forEach(btn => {
        btn.addEventListener("click", () => {
            $all("#mainTabs .nav-link").forEach(b => b.classList.remove("active"));
            btn.classList.add("active");
            $all(".tab-pane").forEach(p => p.classList.add("d-none"));
            $(`#tab-${btn.dataset.tab}`).classList.remove("d-none");
            updatePreview();
        });
    });

    function base() { return $("#baseUrl").value.trim().replace(/\/+$/, ""); }
    function db() { return $("#cfgDatabase").value.trim(); }
    function schema() { return $("#cfgSchema").value.trim(); }
    function table() { return $("#cfgTable").value.trim(); }
    function tableEndpoint() { return `/${db() || "{database}"}/${schema() || "{schema}"}/${table() || "{table}"}`; }

    function refreshEndpointPreviews() {
        $("#baseEndpointPreview").textContent = tableEndpoint();
        $("#crudEndpointLabel").textContent = tableEndpoint();
        saveSettings();
        updatePreview();
    }

    ["baseUrl", "cfgDatabase", "cfgSchema", "cfgTable"].forEach(id => {
        $("#" + id).addEventListener("input", refreshEndpointPreviews);
    });

    $("#authType").addEventListener("change", () => {
        const authType = $("#authType").value;
        $("#authBearerFields").classList.toggle("d-none", authType !== "bearer");
        $("#authBasicFields").classList.toggle("d-none", authType !== "basic");
        saveSettings();
        updatePreview();
    });

    ["authToken", "authUser", "authPass"].forEach(id => {
        $("#" + id).addEventListener("input", () => {
            saveSettings();
            updatePreview();
        });
    });

    function authHeaders() {
        const authType = $("#authType").value;
        if (authType === "bearer" && $("#authToken").value.trim()) {
            return { Authorization: "Bearer " + $("#authToken").value.trim() };
        }
        if (authType === "basic" && $("#authUser").value.trim()) {
            return { Authorization: "Basic " + btoa($("#authUser").value.trim() + ":" + $("#authPass").value) };
        }
        return {};
    }

    function addHeaderRow(k = "", v = "") {
        const row = el("div", "row-card d-flex gap-2 align-items-center");
        row.innerHTML = `
    <input class="form-control form-control-sm font-mono" placeholder="Nom" style="flex:1" value="${escapeAttr(k)}">
    <input class="form-control form-control-sm font-mono" placeholder="Valeur" style="flex:1.4" value="${escapeAttr(v)}">
    <button class="row-remove"><i class="bi bi-x-lg"></i></button>`;
        row.querySelector(".row-remove").addEventListener("click", () => {
            row.remove();
            updatePreview();
        });
        row.querySelectorAll("input").forEach(input => input.addEventListener("input", updatePreview));
        $("#headerRows").appendChild(row);
    }

    $("#btnAddHeader").addEventListener("click", () => addHeaderRow());

    function customHeaders() {
        const out = {};
        $all("#headerRows .row-card").forEach(row => {
            const [k, v] = row.querySelectorAll("input");
            if (k.value.trim()) {
                out[k.value.trim()] = v.value;
            }
        });
        return out;
    }

    function escapeAttr(s) { return String(s).replace(/"/g, "&quot;"); }

    $("#btnHealth").addEventListener("click", async () => {
        const pill = $("#healthPill");
        pill.className = "status-pill";
        pill.innerHTML = '<span class="status-dot"></span> test en cours…';
        try {
            const r1 = await fetch(base() + "/_health", { headers: { ...authHeaders(), ...customHeaders() } });
            const r2 = await fetch(base() + "/_ready", { headers: { ...authHeaders(), ...customHeaders() } });
            if (r1.ok && r2.ok) {
                pill.className = "status-pill ok";
                pill.innerHTML = '<span class="status-dot"></span> en ligne (health + ready)';
            } else {
                pill.className = "status-pill ko";
                pill.innerHTML = `<span class="status-dot"></span> health:${r1.status} ready:${r2.status}`;
            }
        } catch (e) {
            pill.className = "status-pill ko";
            pill.innerHTML = '<span class="status-dot"></span> injoignable';
        }
    });

    $all(".quick-btn").forEach(btn => {
        btn.addEventListener("click", async () => {
            let path;
            switch (btn.dataset.quick) {
                case "health": path = "/_health"; break;
                case "ready": path = "/_ready"; break;
                case "databases": path = "/databases"; break;
                case "schemas": path = "/schemas"; break;
                case "tables": path = "/tables"; break;
                case "schematables": path = `/${db() || "{database}"}/${schema() || "{schema}"}`; break;
                case "show": path = `/show/${db() || "{database}"}/${schema() || "{schema}"}/${table() || "{table}"}`; break;
            }
            await runRequest("GET", path, null);
        });
    });

    const OPERATORS = [
        ["", "= (égalité directe)"],
        ["$eq", "$eq — égal à"],
        ["$gt", "$gt — supérieur à"],
        ["$gte", "$gte — supérieur ou égal"],
        ["$lt", "$lt — inférieur à"],
        ["$lte", "$lte — inférieur ou égal"],
        ["$ne", "$ne — différent de"],
        ["$in", "$in — dans la liste (a,b,c)"],
        ["$nin", "$nin — absent de la liste"],
        ["$null", "$null — est nul"],
        ["$notnull", "$notnull — n'est pas nul"],
        ["$true", "$true — est vrai"],
        ["$nottrue", "$nottrue — n'est pas vrai"],
        ["$false", "$false — est faux"],
        ["$notfalse", "$notfalse — n'est pas faux"],
        ["$like", "$like — motif (sensible à la casse)"],
        ["$ilike", "$ilike — motif (insensible à la casse)"],
        ["$nlike", "$nlike — exclut le motif"],
        ["$nilike", "$nilike — exclut le motif (insensible)"],
        ["$ltreelanc", "$ltreelanc — ancêtre ltree"],
        ["$ltreerdesc", "$ltreerdesc — descendant ltree"],
        ["$ltreematch", "$ltreematch — correspond à lquery"],
        ["$ltreematchtxt", "$ltreematchtxt — correspond à ltxtquery"],
    ];
    const NO_VALUE_OPS = new Set(["$null", "$notnull", "$true", "$nottrue", "$false", "$notfalse"]);

    function operatorOptions(selected = "") {
        return OPERATORS.map(([v, label]) => `<option value="${v}" ${v === selected ? "selected" : ""}>${label}</option>`).join("");
    }

    function makeFilterRow(container, field = "", op = "", value = "") {
        const row = el("div", "row-card");
        row.innerHTML = `
    <div class="d-flex gap-2 align-items-start flex-wrap">
      <input class="form-control form-control-sm font-mono flt-field" placeholder="champ" style="flex:1.1;min-width:110px" value="${escapeAttr(field)}">
      <select class="form-select form-select-sm font-mono flt-op" style="flex:1.4;min-width:180px">${operatorOptions(op)}</select>
      <input class="form-control form-control-sm font-mono flt-val" placeholder="valeur" style="flex:1.3;min-width:110px" value="${escapeAttr(value)}">
      <button class="row-remove"><i class="bi bi-x-lg"></i></button>
    </div>`;
        const valInput = row.querySelector(".flt-val");
        const opSelect = row.querySelector(".flt-op");

        function syncValueVisibility() {
            valInput.classList.toggle("d-none", NO_VALUE_OPS.has(opSelect.value));
        }

        syncValueVisibility();
        opSelect.addEventListener("change", () => {
            syncValueVisibility();
            updatePreview();
        });
        row.querySelectorAll("input").forEach(input => input.addEventListener("input", updatePreview));
        row.querySelector(".row-remove").addEventListener("click", () => {
            row.remove();
            updatePreview();
        });
        container.appendChild(row);
    }

    function readFilterRows(container) {
        return $all(".row-card", container).map(row => {
            const field = row.querySelector(".flt-field").value.trim();
            const op = row.querySelector(".flt-op").value;
            const value = row.querySelector(".flt-val").value;
            return { field, op, value };
        }).filter(f => f.field);
    }

    function filterToQueryPair(f) {
        if (!f.field) return null;
        if (NO_VALUE_OPS.has(f.op)) return [f.field, f.op];
        if (!f.op) return [f.field, f.value];
        return [f.field, `${f.op}.${f.value}`];
    }

    $("#btnAddFilter").addEventListener("click", () => makeFilterRow($("#filterRows")));
    $("#btnAddFilterWrite").addEventListener("click", () => makeFilterRow($("#filterRowsWrite")));
    makeFilterRow($("#filterRows"));

    function makeOrRow(container, field = "", op = "$eq", value = "") {
        const row = el("div", "row-card");
        row.innerHTML = `
    <div class="d-flex gap-2 align-items-start flex-wrap">
      <input class="form-control form-control-sm font-mono flt-field" placeholder="champ" style="flex:1.1;min-width:110px" value="${escapeAttr(field)}">
      <select class="form-select form-select-sm font-mono flt-op" style="flex:1.4;min-width:180px">${operatorOptions(op)}</select>
      <input class="form-control form-control-sm font-mono flt-val" placeholder="valeur" style="flex:1.3;min-width:110px" value="${escapeAttr(value)}">
      <button class="row-remove"><i class="bi bi-x-lg"></i></button>
    </div>`;
        row.querySelectorAll("input,select").forEach(input => input.addEventListener("input", updatePreview));
        row.querySelector(".flt-op").addEventListener("change", updatePreview);
        row.querySelector(".row-remove").addEventListener("click", () => {
            row.remove();
            updatePreview();
        });
        container.appendChild(row);
    }

    $("#btnAddOr").addEventListener("click", () => makeOrRow($("#orRows")));

    function readOrGroup() {
        const alts = $all(".row-card", $("#orRows")).map(row => {
            const field = row.querySelector(".flt-field").value.trim();
            const op = row.querySelector(".flt-op").value || "$eq";
            const value = row.querySelector(".flt-val").value;
            if (!field) return null;
            return NO_VALUE_OPS.has(op) ? `${field}=${op}` : `${field}=${op}.${value}`;
        }).filter(Boolean);
        return alts.length ? alts.join("||") : null;
    }

    let pendingJoin = null;

    function refreshCrudMode() {
        const method = $("#crudMethod").value;
        const isGet = method === "GET";
        $("#crudGetBuilder").classList.toggle("d-none", !isGet);
        $("#crudBodyBuilder").classList.toggle("d-none", isGet);
        $("#crudFilterForWrite").classList.toggle("d-none", method === "POST");
        if (method === "POST") {
            $("#bodyHint").innerHTML = 'POST envoie un <span class="op-code">INSERT</span> — le corps JSON représente la ligne à créer.';
        } else if (method === "DELETE") {
            $("#bodyHint").innerHTML = 'DELETE ignore le corps — utilisez les filtres ci-dessous pour cibler les lignes via <span class="op-code">WHERE</span>. Sans filtre, <strong>toutes les lignes</strong> seraient supprimées.';
            $("#bodyJson").closest("#crudBodyBuilder").querySelector("#bodyJson").classList.add("d-none");
            $("#btnFormatBody").classList.add("d-none");
        } else {
            $("#bodyHint").innerHTML = `${method} envoie un <span class="op-code">UPDATE</span> — le corps JSON contient les champs à modifier, et les filtres ci-dessous forment le <span class="op-code">WHERE</span>. Sans filtre, <strong>toutes les lignes</strong> seraient modifiées.`;
            $("#bodyJson").classList.remove("d-none");
            $("#btnFormatBody").classList.remove("d-none");
        }
        if (method !== "DELETE") {
            $("#bodyJson").classList.remove("d-none");
            $("#btnFormatBody").classList.remove("d-none");
        }
        updatePreview();
    }

    $("#crudMethod").addEventListener("change", refreshCrudMode);
    refreshCrudMode();

    $("#btnFormatBody").addEventListener("click", () => {
        try {
            const v = JSON.parse($("#bodyJson").value || "{}");
            $("#bodyJson").value = JSON.stringify(v, null, 2);
        } catch (e) {
            toast("JSON invalide : " + e.message);
        }
    });
    $("#bodyJson").addEventListener("input", updatePreview);

    function buildCrudGetQuery() {
        const params = [];
        readFilterRows($("#filterRows")).forEach(f => {
            const pair = filterToQueryPair(f);
            if (pair) {
                params.push(`${encodeURIComponent(pair[0])}=${encodeURIComponent(pair[1])}`);
            }
        });
        const orGroup = readOrGroup();
        if (orGroup) {
            params.push(`_or=${encodeURIComponent(orGroup)}`);
        }

        const page = $("#optPage").value.trim();
        const pageSize = $("#optPageSize").value.trim();
        const order = $("#optOrder").value.trim();
        const groupby = $("#optGroupby").value.trim();
        const select = $("#optSelect").value.trim();
        const count = $("#optCount").value.trim();
        const renderer = $("#optRenderer").value;
        if (page) params.push(`_page=${encodeURIComponent(page)}`);
        if (pageSize) params.push(`_page_size=${encodeURIComponent(pageSize)}`);
        if (order) params.push(`_order=${encodeURIComponent(order)}`);
        if (groupby) params.push(`_groupby=${encodeURIComponent(groupby)}`);
        if (select) params.push(`_select=${encodeURIComponent(select.split(",").map(s => s.trim()).join(","))}`);
        if (count) params.push(`_count=${encodeURIComponent(count)}`);
        if (renderer) params.push(`_renderer=${renderer}`);
        if ($("#optDistinct").checked) params.push("_distinct=true");
        if ($("#optCountFirst").checked) params.push("_count_first=true");
        return params;
    }

    $all("#tab-crud input, #tab-crud select").forEach(input => input.addEventListener("input", updatePreview));
    $all("#tab-crud select").forEach(input => input.addEventListener("change", updatePreview));

    function goToCrudGet() {
        $("#mainTabs .nav-link[data-tab='crud']").click();
        $("#crudMethod").value = "GET";
        refreshCrudMode();
    }

    function ensureCrudTarget(requireTable = true) {
        if (!db()) {
            toast("Renseignez la base cible.");
            $("#cfgDatabase").focus();
            return false;
        }
        if (!schema()) {
            toast("Renseignez le schéma cible.");
            $("#cfgSchema").focus();
            return false;
        }
        if (requireTable && !table()) {
            toast("Renseignez la table ou la vue cible.");
            $("#cfgTable").focus();
            return false;
        }
        return true;
    }

    $("#btnUseRange").addEventListener("click", () => {
        const field = $("#rangeField").value.trim();
        const from = $("#rangeFrom").value.trim();
        const to = $("#rangeTo").value.trim();
        if (!field || !from || !to) {
            toast("Renseignez le champ, la borne de début et de fin.");
            return;
        }
        makeFilterRow($("#filterRows"), field, "$gte", from);
        makeFilterRow($("#filterRows"), field, "$lte", to);
        goToCrudGet();
        toast("Filtre de plage ajouté à la requête CRUD.");
    });

    $("#btnUseJoin").addEventListener("click", () => {
        const type = $("#joinType").value;
        const joinTable = $("#joinTable").value.trim();
        const field1 = $("#joinField1").value.trim();
        const op = $("#joinOp").value;
        const field2 = $("#joinField2").value.trim();
        if (!joinTable || !field1 || !field2) {
            toast("Renseignez la table jointe et les deux champs.");
            return;
        }
        pendingJoin = `${type}:${joinTable}:${field1}:${op}:${field2}`;
        goToCrudGet();
        updatePreview();
        toast("JOIN ajouté à la requête CRUD.");
    });

    $("#btnUseJsonb").addEventListener("click", () => {
        const field = $("#jsonbField").value.trim();
        const key = $("#jsonbKey").value.trim();
        const value = $("#jsonbValue").value.trim();
        if (!field || !key) {
            toast("Renseignez le champ JSONB et la clé.");
            return;
        }
        makeFilterRow($("#filterRows"), `${field}->>${key}:jsonb`, "", value);
        goToCrudGet();
        toast("Filtre JSONB ajouté à la requête CRUD.");
    });

    $("#btnUseTsquery").addEventListener("click", () => {
        const field = $("#tsField").value.trim();
        const lang = $("#tsLang").value;
        const value = $("#tsValue").value.trim();
        if (!field || !value) {
            toast("Renseignez le champ et la valeur tsquery.");
            return;
        }
        const fieldExpr = lang ? `${field}$${lang}:tsquery` : `${field}:tsquery`;
        makeFilterRow($("#filterRows"), fieldExpr, "", value);
        goToCrudGet();
        toast("Recherche plein texte ajoutée à la requête CRUD.");
    });

    $("#btnCrudSend").addEventListener("click", async () => {
        if (!ensureCrudTarget()) {
            return;
        }
        const method = $("#crudMethod").value;
        const path = tableEndpoint();
        let query = [];
        let body;

        if (method === "GET") {
            query = buildCrudGetQuery();
            if (pendingJoin) {
                query.push(`_join=${encodeURIComponent(pendingJoin)}`);
            }
        } else {
            if (method !== "POST") {
                readFilterRows($("#filterRowsWrite")).forEach(f => {
                    const pair = filterToQueryPair(f);
                    if (pair) {
                        query.push(`${encodeURIComponent(pair[0])}=${encodeURIComponent(pair[1])}`);
                    }
                });
            }
            if (method !== "DELETE") {
                try {
                    body = $("#bodyJson").value.trim() ? JSON.parse($("#bodyJson").value) : {};
                } catch (e) {
                    toast("Corps JSON invalide : " + e.message);
                    return;
                }
            }
        }
        await runRequest(method, path, body, query);
    });

    $("#btnFormatBatch").addEventListener("click", () => {
        try {
            const v = JSON.parse($("#batchJson").value || "[]");
            $("#batchJson").value = JSON.stringify(v, null, 2);
        } catch (e) {
            toast("JSON invalide : " + e.message);
        }
    });

    $("#btnSendBatch").addEventListener("click", async () => {
        if (!ensureCrudTarget()) {
            return;
        }
        let body;
        try {
            body = JSON.parse($("#batchJson").value || "[]");
        } catch (e) {
            toast("Corps JSON invalide : " + e.message);
            return;
        }
        const extraHeaders = {};
        if ($("#batchCopyMethod").checked) {
            extraHeaders["Prest-Batch-Method"] = "copy";
        }
        await runRequest("POST", `/batch/${db() || "{database}"}/${schema() || "{schema}"}/${table() || "{table}"}`, body, [], extraHeaders);
    });

    ["batchJson", "batchCopyMethod"].forEach(id => $("#" + id).addEventListener("input", updatePreview));

    function customTargetPath() {
        return `/_QUERIES/${$("#customFolder").value.trim() || "{dossier}"}/${$("#customName").value.trim() || "{nom}"}`;
    }

    function refreshCustomEndpoint() {
        $("#customEndpointLabel").textContent = customTargetPath();
        $("#customBodyWrap").classList.toggle("d-none", $("#customMethod").value === "GET" || $("#customMethod").value === "DELETE");
        updatePreview();
    }

    ["customFolder", "customName"].forEach(id => $("#" + id).addEventListener("input", refreshCustomEndpoint));
    $("#customMethod").addEventListener("change", refreshCustomEndpoint);

    function makeCustomParamRow(k = "", v = "") {
        const row = el("div", "row-card d-flex gap-2 align-items-center");
        row.innerHTML = `
    <input class="form-control form-control-sm font-mono" placeholder="clé (ex: field1)" style="flex:1" value="${escapeAttr(k)}">
    <input class="form-control form-control-sm font-mono" placeholder="valeur" style="flex:1.4" value="${escapeAttr(v)}">
    <button class="row-remove"><i class="bi bi-x-lg"></i></button>`;
        row.querySelector(".row-remove").addEventListener("click", () => {
            row.remove();
            updatePreview();
        });
        row.querySelectorAll("input").forEach(input => input.addEventListener("input", updatePreview));
        $("#customParamRows").appendChild(row);
    }

    $("#btnAddCustomParam").addEventListener("click", () => makeCustomParamRow());
    makeCustomParamRow();
    refreshCustomEndpoint();

    $("#customBody").addEventListener("input", updatePreview);

    $("#btnSendCustom").addEventListener("click", async () => {
        const method = $("#customMethod").value;
        const params = $all("#customParamRows .row-card").map(row => {
            const [k, v] = row.querySelectorAll("input");
            return [k.value.trim(), v.value];
        }).filter(([k]) => k);
        const query = params.map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v)}`);
        let body;
        if (method !== "GET" && method !== "DELETE") {
            try {
                body = $("#customBody").value.trim() ? JSON.parse($("#customBody").value) : {};
            } catch (e) {
                toast("Corps JSON invalide : " + e.message);
                return;
            }
        }
        await runRequest(method, customTargetPath(), body, query);
    });

    function updatePreview() {
        const activeTab = $("#mainTabs .nav-link.active")?.dataset.tab;
        let method;
        let path;
        let query = [];
        let body;

        if (activeTab === "custom") {
            method = $("#customMethod").value;
            path = customTargetPath();
            query = $all("#customParamRows .row-card").map(row => {
                const [k, v] = row.querySelectorAll("input");
                return k.value.trim() ? `${encodeURIComponent(k.value.trim())}=${encodeURIComponent(v.value)}` : null;
            }).filter(Boolean);
            if (method !== "GET" && method !== "DELETE") {
                body = $("#customBody").value;
            }
        } else if (activeTab === "advanced") {
            method = "POST";
            path = `/batch/${db() || "{database}"}/${schema() || "{schema}"}/${table() || "{table}"}`;
            body = $("#batchJson").value;
        } else {
            method = $("#crudMethod").value;
            path = tableEndpoint();
            if (method === "GET") {
                query = buildCrudGetQuery();
                if (pendingJoin) {
                    query.push(`_join=${encodeURIComponent(pendingJoin)}`);
                }
            } else {
                if (method !== "POST") {
                    readFilterRows($("#filterRowsWrite")).forEach(f => {
                        const pair = filterToQueryPair(f);
                        if (pair) {
                            query.push(`${encodeURIComponent(pair[0])}=${encodeURIComponent(pair[1])}`);
                        }
                    });
                }
                if (method !== "DELETE") {
                    body = $("#bodyJson").value;
                }
            }
        }

        const url = base() + path + (query.length ? "?" + query.join("&") : "");
        const headers = { ...authHeaders(), ...customHeaders() };
        if (activeTab === "advanced" && $("#batchCopyMethod").checked) {
            headers["Prest-Batch-Method"] = "copy";
        }
        if (body !== undefined) {
            headers["Content-Type"] = "application/json";
        }

        let cmd = `<span class="tok-cmd">curl</span> <span class="tok-flag">-i -X ${method}</span> <span class="tok-url">'${escapeHtml(url)}'</span>`;
        Object.entries(headers).forEach(([k, v]) => {
            cmd += ` \\
  <span class="tok-flag">-H</span> '${escapeHtml(k)}: ${escapeHtml(v)}'`;
        });
        if (body !== undefined && body !== null && String(body).trim() !== "") {
            cmd += ` \\
  <span class="tok-flag">-d</span> '${escapeHtml(String(body))}'`;
        }
        $("#curlPreview").innerHTML = cmd;
    }

    function escapeHtml(s) {
        return String(s).replace(/[&<>]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" }[c]));
    }

    function setupSidebarResizer() {
        const layout = $(".layout");
        const sidebar = $("#sidebar");
        const resizer = $("#layoutResizer");
        if (!layout || !sidebar || !resizer) {
            return;
        }

        const mobileMq = window.matchMedia("(max-width: 991.98px)");
        const minWidth = 240;

        function maxWidth() {
            return Math.max(minWidth, Math.min(720, layout.getBoundingClientRect().width - 320));
        }

        function clampWidth(nextWidth) {
            return Math.min(Math.max(nextWidth, minWidth), maxWidth());
        }

        function applyWidth(nextWidth, shouldPersist = false) {
            if (mobileMq.matches) {
                layout.style.setProperty("--sidebar-width", "100%");
                return;
            }
            const clamped = clampWidth(nextWidth);
            layout.style.setProperty("--sidebar-width", `${clamped}px`);
            if (shouldPersist) {
                persistSettings({ sidebarWidth: clamped });
            }
        }

        let isResizing = false;

        function stopResize() {
            if (!isResizing) {
                return;
            }
            isResizing = false;
            layout.classList.remove("is-resizing");
            persistSettings({ sidebarWidth: sidebar.offsetWidth });
            window.removeEventListener("mousemove", onMouseMove);
            window.removeEventListener("mouseup", stopResize);
        }

        function onMouseMove(event) {
            const bounds = layout.getBoundingClientRect();
            applyWidth(event.clientX - bounds.left);
        }

        resizer.addEventListener("mousedown", event => {
            if (mobileMq.matches) {
                return;
            }
            event.preventDefault();
            isResizing = true;
            layout.classList.add("is-resizing");
            window.addEventListener("mousemove", onMouseMove);
            window.addEventListener("mouseup", stopResize);
        });

        resizer.addEventListener("keydown", event => {
            if (mobileMq.matches) {
                return;
            }
            if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") {
                return;
            }
            event.preventDefault();
            const direction = event.key === "ArrowRight" ? 1 : -1;
            applyWidth(sidebar.offsetWidth + direction * 24, true);
        });

        mobileMq.addEventListener("change", () => {
            if (mobileMq.matches) {
                stopResize();
                layout.style.setProperty("--sidebar-width", "100%");
                return;
            }
            applyWidth(sidebar.offsetWidth, false);
        });

        window.addEventListener("resize", () => {
            if (!mobileMq.matches) {
                applyWidth(sidebar.offsetWidth, false);
            }
        });

        const savedWidth = Number(loadSettings().sidebarWidth);
        applyWidth(Number.isFinite(savedWidth) && savedWidth > 0 ? savedWidth : sidebar.offsetWidth, false);
    }

    $("#btnCopyCurl").addEventListener("click", () => {
        navigator.clipboard.writeText($("#curlPreview").textContent).then(() => toast("Commande cURL copiée."));
    });

    async function runRequest(method, path, body, extraQuery = [], extraHeaders = {}) {
        const url = base() + path + (extraQuery.length ? "?" + extraQuery.join("&") : "");
        const headers = { ...authHeaders(), ...customHeaders(), ...extraHeaders };
        const opts = { method, headers };
        if (body !== undefined && method !== "GET" && method !== "DELETE") {
            headers["Content-Type"] = "application/json";
            opts.body = JSON.stringify(body);
        }

        $("#respStatus").textContent = "…";
        $("#respStatus").className = "resp-status";
        $("#respMeta").textContent = "requête en cours…";
        const t0 = performance.now();

        try {
            const resp = await fetch(url, opts);
            const elapsed = Math.round(performance.now() - t0);
            const statusClass = resp.status >= 500 ? "resp-5xx" : resp.status >= 400 ? "resp-4xx" : "resp-2xx";
            $("#respStatus").textContent = `${resp.status} ${resp.statusText}`;
            $("#respStatus").className = "resp-status " + statusClass;
            $("#respMeta").textContent = `${method} ${path} · ${elapsed} ms`;

            const headersObj = {};
            resp.headers.forEach((v, k) => {
                headersObj[k] = v;
            });
            if (Object.keys(headersObj).length) {
                $("#respHeadersAccordion").style.display = "";
                $("#respHeadersContent").innerHTML = Object.entries(headersObj)
                    .map(([k, v]) => `<div><span style="color:#7fd0ff">${escapeHtml(k)}</span>: ${escapeHtml(v)}</div>`)
                    .join("");
            }

            const text = await resp.text();
            renderResponseBody(text);
        } catch (e) {
            $("#respStatus").textContent = "réseau";
            $("#respStatus").className = "resp-status resp-err";
            $("#respMeta").textContent = `${method} ${path} · échec`;
            $("#respBody").textContent = `Erreur réseau : ${e.message}\n\nVérifiez l'URL de base, le CORS côté serveur prestd (en-tête Access-Control-Allow-Origin), et la connectivité.`;
        }
    }

    function renderResponseBody(text) {
        if (!text) {
            $("#respBody").textContent = "// Réponse vide";
            return;
        }
        try {
            const json = JSON.parse(text);
            $("#respBody").innerHTML = syntaxHighlight(JSON.stringify(json, null, 2));
        } catch (e) {
            $("#respBody").textContent = text;
        }
    }

    function syntaxHighlight(json) {
        const escapedJson = escapeHtml(json);
        return escapedJson.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false)\b|\bnull\b|-?\d+(?:\.\d*)?(?:[eE][+-]?\d+)?)/g,
            match => {
                let cls = "jv-num";
                if (/^"/.test(match)) {
                    cls = /:$/.test(match) ? "jv-key" : "jv-str";
                } else if (/true|false/.test(match)) {
                    cls = "jv-bool";
                } else if (/null/.test(match)) {
                    cls = "jv-null";
                }
                return `<span class="${cls}">${match}</span>`;
            });
    }

    $("#btnCopyResp").addEventListener("click", () => {
        navigator.clipboard.writeText($("#respBody").textContent).then(() => toast("Réponse copiée."));
    });

    $("#btnClearResp").addEventListener("click", () => {
        $("#respBody").textContent = "// La réponse formatée apparaîtra ici après l'envoi d'une requête.";
        $("#respStatus").textContent = "—";
        $("#respStatus").className = "resp-status";
        $("#respMeta").textContent = "aucune requête envoyée";
        $("#respHeadersAccordion").style.display = "none";
    });

    (function init() {
        const s = loadSettings();
        if (s.baseUrl) $("#baseUrl").value = s.baseUrl;
        if (s.database) $("#cfgDatabase").value = s.database;
        if (s.schema) $("#cfgSchema").value = s.schema;
        if (s.table) $("#cfgTable").value = s.table;
        if (s.authType) {
            $("#authType").value = s.authType;
            $("#authType").dispatchEvent(new Event("change"));
        }
        if (s.authToken) $("#authToken").value = s.authToken;
        if (s.authUser) $("#authUser").value = s.authUser;
        if (s.authPass) $("#authPass").value = s.authPass;
        setupSidebarResizer();
        refreshEndpointPreviews();
        updatePreview();
    })();
})();