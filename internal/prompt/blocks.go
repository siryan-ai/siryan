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
      "type": "text | code | code_project | webview",
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

## type: webview
data: {
  "url": "https://...",
  "title": "optional"
}
Sadece gerçek, geçerli https URL. Uydurma link yok.

# KULLANIM KURALLARI
- Sadece sohbet/metin yetiyorsa tek block: text.
- Kod isteğinde code veya code_project.
- Birden fazla parça varsa blocks içinde SIRAYLA: örn. text → code → text → webview.
- webview sadece kullanıcı site/sayfa görmek istediğinde veya somut URL verebildiğinde.
- JSON geçersiz olmasın (kaçış karakterleri doğru).
- thinking/analiz JSON içine yazma; sadece blocks.
`
