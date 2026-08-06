package email

import "fmt"

// Layout оборачивает содержимое письма в фирменную рамку Amida.
// Таблицы и inline-стили сознательно: Outlook рендерит письма движком Word
// (ни flex, ни grid, ни border-radius на div), Gmail местами вырезает <style>.
// table + style="..." — то немногое, что показывают одинаково все клиенты.
//
// heading и contentHTML попадают в разметку как есть — вызывающий обязан
// экранировать всё, что пришло от пользователя (см. welcomeBody).
func Layout(heading, contentHTML string) string {
	return fmt.Sprintf(`<div style="background:#f4f4f5;padding:32px 16px;font-family:-apple-system,'Segoe UI',Roboto,Helvetica,Arial,sans-serif">
<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="max-width:480px;margin:0 auto;border-collapse:collapse">
<tr><td style="background:#18181b;padding:20px 32px;border-radius:12px 12px 0 0">
<span style="color:#fafafa;font-size:18px;font-weight:600;letter-spacing:-0.02em">Amida</span>
</td></tr>
<tr><td style="background:#ffffff;padding:32px">
<h1 style="margin:0 0 16px;font-size:20px;line-height:1.3;font-weight:600;color:#18181b">%s</h1>
%s
</td></tr>
<tr><td style="background:#ffffff;padding:16px 32px;border-top:1px solid #e4e4e7;border-radius:0 0 12px 12px">
<p style="margin:0;font-size:12px;line-height:1.5;color:#71717a">Amida — платформа для репетиторов · <a href="https://amida.kz" style="color:#71717a">amida.kz</a></p>
</td></tr>
</table>
</div>`, heading, contentHTML)
}

// Button — CTA-ссылка, выглядящая кнопкой. Отдельно от Layout, потому что нужна
// не каждому письму: в OTP кнопки нет, код копируют глазами.
func Button(href, label string) string {
	return fmt.Sprintf(`<a href="%s" style="display:inline-block;background:#18181b;color:#fafafa;`+
		`text-decoration:none;padding:12px 24px;border-radius:8px;font-size:15px;font-weight:500">%s</a>`,
		href, label)
}
