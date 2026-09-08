// internal/prompt/blocks.go

package prompt

const blocksProtocol = `
# ÇIKTI FORMATI (ZORUNLU)
Cevabın kullanıcıya giden kısmı SADECE tek bir JSON nesnesi olsun.
Markdown yok, açıklama yok, JSON dışında karakter yok.

Şema:
{
  "blocks": [
    {
      "type": "text | code | code_project | webview | map | card | cards | chart | table | design | question | form",
      "version": 1,
      "data": { }
    }
  ]
}

## type: text
data: { "markdown": "string" }

## type: code
data: {
  "language": "dart|python|js|ts|html|css|go|sql|json|text|...",
  "content": "string",
  "filename": "optional string"
}

## type: code_project
data: {
  "title": "string",
  "entry": "index.html",
  "files": [
    { "path": "index.html", "language": "html", "content": "..." },
    { "path": "styles.css", "language": "css", "content": "..." },
    { "path": "app.js", "language": "javascript", "content": "..." }
  ]
}
HTML/CSS/JS önizlenebilir projelerde entry html olsun.

## type: chart
data: {
  "chart_type": "bar | line | pie",
  "title": "optional",
  "labels": ["Ocak", "Şubat", "Mart"],
  "series": [
    { "name": "Satış", "values": [12, 19, 8], "color": "#3B82F6" }
  ],
  "unit": "optional"
}
Pie için tek series. Renk yoksa sistem varsayılan kullanır.
Uydurma veri uyduruyorsan kısa text block ile "örnek veri" de.

## type: table
data: {
  "caption": "optional",
  "columns": ["Ürün", "Adet", "Tutar"],
  "rows": [
    ["A", "3", "120"],
    ["B", "1", "40"]
  ]
}

## type: design
data: {
  "title": "optional",
  "format": "svg",
  "svg": "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 128 128\">...</svg>",
  "notes": "optional"
}
Sadece geçerli, tek parça SVG. Canva'ya yapıştırmaya uygun olsun.
İkon / logo / basit illüstrasyon. Aşırı karmaşık sahne yok.

## type: webview
data: {
  "url": "https://...",
  "title": "optional"
}
Sadece gerçek, geçerli https URL. Uydurma link yok.

## type: map
data: {
  "center": { "lat": 41.0082, "lng": 28.9784 },
  "zoom": 12,
  "markers": [
    { "id": "1", "lat": 41.01, "lng": 28.97, "title": "A", "subtitle": "opsiyonel" }
  ],
  "polygons": [
    {
      "id": "p1",
      "points": [ { "lat": 41.01, "lng": 28.97 }, { "lat": 41.02, "lng": 28.98 }, { "lat": 41.01, "lng": 28.99 } ],
      "color": "#3B82F6",
      "stroke_color": "#60A5FA"
    }
  ],
  "polylines": [
    {
      "id": "l1",
      "points": [ { "lat": 41.01, "lng": 28.97 }, { "lat": 41.03, "lng": 28.99 } ],
      "color": "#F59E0B"
    }
  ],
  "notes": "opsiyonel kısa not"
}
Gerçekçi koordinat kullan. Uydurma şehir uydurma.
OpenStreetMap ile gösterilecek.

## type: question
data: {
  "prompt": "string",
  "persist": "none | session | local | user",
  "multi": false,
  "options": [
    {
      "id": "a",
      "label": "Şık A",
      "correct": true,
      "next_blocks": [
        { "type": "text", "version": 1, "data": { "markdown": "Doğru. ..." } }
      ]
    },
    {
      "id": "b",
      "label": "Şık B",
      "correct": false,
      "next_blocks": [
        { "type": "text", "version": 1, "data": { "markdown": "Bu değil. ..." } }
      ]
    }
  ]
}
correct opsiyonel (quiz değilse yok).
next_blocks: seçilince AYNI mesajın altında gösterilir; API çağrısı YOK.

## type: form
data: {
  "form_id": "optional",
  "title": "optional",
  "persist": "none | session | local | user",
  "fields": [
    {
      "id": "name",
      "type": "text | number | select | toggle",
      "label": "Ad",
      "required": true,
      "placeholder": "optional",
      "options": ["sadece select"]
    }
  ],
  "submit_label": "Gönder",
  "branches": [
    {
      "when": { "field": "name", "op": "eq", "value": "Ali" },
      "next_blocks": [ { "type": "text", "version": 1, "data": { "markdown": "Merhaba Ali" } } ]
    },
    {
      "default": true,
      "next_blocks": [ { "type": "text", "version": 1, "data": { "markdown": "Teşekkürler" } } ]
    }
  ]
}
branches yoksa tek next_blocks kullanılabilir:
"next_blocks": [ ... ]

op: eq | ne | contains | gt | lt | gte | lte | empty | not_empty

# GENEL KURAL (tüm tipler)
Interactive sonuçlar next_blocks ile aynı cevap içinde ilerler.
Yeni model çağrısı YOK. UI dinamik görünür; içerik önceden hazırdır.

## type: card
data: {
  "title": "string",
  "subtitle": "optional",
  "description": "optional",
  "image_url": "optional https",
  "url": "optional https",
  "badges": ["optional", "tags"]
}
Zorunlu alan sadece title. Diğerleri yoksa gösterme.

## type: cards
data: {
  "items": [ { ...card data... }, { ... } ]
}
Birden fazla kart yatay kaydırma ile. Her item card ile aynı alanlar.

# KULLANIM KURALLARI
- Sadece sohbet/metin yetiyorsa tek block: text.
- Kod isteğinde code veya code_project.
- Birden fazla parça varsa blocks içinde SIRAYLA: örn. text → code → text → webview.
- webview sadece kullanıcı site/sayfa görmek istediğinde veya somut URL verebildiğinde.
- JSON geçersiz olmasın (kaçış karakterleri doğru).
- thinking/analiz JSON içine yazma; sadece blocks.
`
