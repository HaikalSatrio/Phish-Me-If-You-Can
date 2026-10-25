package personname

import (
	"strings"
	"testing"
)

func TestExplicitPublicNames(t *testing.T) {
	cases := []struct{ subject, text, want string }{
		{"IShowSpeed", "Darren Jason Watkins Jr. (born January 21, 2005), better known as IShowSpeed or simply Speed, is an American influencer.", "Darren Jason Watkins Jr."},
		{"PewDiePie", "Felix Arvid Ulf Kjellberg (born 24 October 1989), known online as PewDiePie, is a Swedish YouTuber.", "Felix Arvid Ulf Kjellberg"},
		{"Rich Brian", "Brian Imanuel Soewarno, known professionally as Rich Brian, is a rapper.", "Brian Imanuel Soewarno"},
		{"Awan", "Sari Utami, dikenal sebagai Awan, adalah penyanyi.", "Sari Utami"},
		{"Lentera", "Ratna Dewi (lahir 1995), yang lebih dikenal dengan nama panggung Lentera, merupakan musisi.", "Ratna Dewi"},
		{"Starling", "Élodie Martin, also known as Starling, is a singer.", "Élodie Martin"},
		{"North", "Liam O'Connor, professionally known as North, is a musician.", "Liam O'Connor"},
		{"Moon", "Anna-Lena Weber, known as Moon, is a writer.", "Anna-Lena Weber"},
		{"Glass", "M. R. Bennett, better known as Glass, is an actor.", "M. R. Bennett"},
		{"Jade", "Mira van der Meer, known as Jade, is a writer.", "Mira van der Meer"},
		{"夜空", "山田太郎, known as 夜空, is a musician.", "山田太郎"},
		{"Nova7", "Aria Bell, known online as Nova7, is a streamer.", "Aria Bell"},
		{"Mono", "Sutrisno, dikenal sebagai Mono, adalah pelawak.", "Sutrisno"},
		{"Drift", "Update. Rafael Costa, known as Drift, is a singer.", "Rafael Costa"},
		{"Ember", "Ember, whose real name is Lina Hart, performs jazz.", "Lina Hart"},
		{"Echo", "Echo's real name is Maya Stone.", "Maya Stone"},
		{"Merah", "Merah, bernama asli Nila Putri, adalah penyanyi.", "Nila Putri"},
		{"Fable", "Fable (real name: Nora Vale) is a writer.", "Nora Vale"},
		{"Biru", "Biru (nama asli Damar Wicaksono) adalah pelawak.", "Damar Wicaksono"},
		{"Silver", "Silver is the stage name of Elias Rivers.", "Elias Rivers"},
		{"Pena", "Pena adalah nama pena dari Ratri Wulandari.", "Ratri Wulandari"},
		{"Sari Utami", "Sari Utami, known as Awan, is a singer.", "Sari Utami"},
		{"Speed", "Darren Jason Watkins Jr., better known as IShowSpeed or simply Speed, is a streamer.", "Darren Jason Watkins Jr."},
		{"Awan", "Sari Utami, known as Awan, is a singer. Tari Murni, known as Bara, is a writer.", "Sari Utami"},
		{"Lentera", "Ratna Dewi, dikenal secara profesional dengan nama Lentera (lahir 1995), adalah penyanyi.", "Ratna Dewi"},
		{"Glass", "Rafael Costa, known professionally as Glass (formerly Crystal), is an actor.", "Rafael Costa"},
		{"Moon", "Anna Weber, known as Moon (/mu:n/), is a writer.", "Anna Weber"},
		{"North", "Liam O'Connor, known by his ring name North, is a wrestler.", "Liam O'Connor"},
		{"Batu", "Rudi Santoso, lebih dikenal dengan nama ring Batu, adalah pegulat.", "Rudi Santoso"},
		{"North", "Liam O'Connor KBE (born 1990), known by his stage name North, is a musician.", "Liam O'Connor"},
		{"North", "Liam O'Connor, also known by his nickname North, is a musician.", "Liam O'Connor"},
		{"Batu", "Rudi Santoso, juga dikenal sebagai Batu, adalah pelawak.", "Rudi Santoso"},
	}
	for _, tc := range cases {
		t.Run(tc.subject+"/"+tc.want, func(t *testing.T) {
			got := Find(tc.subject, tc.text)
			if len(got) != 1 || got[0].Name != tc.want || !strings.Contains(tc.text, got[0].Quote) {
				t.Fatalf("want %q with literal source, got %+v", tc.want, got)
			}
		})
	}
}

func TestNameAbstentions(t *testing.T) {
	cases := []struct{ subject, text string }{
		{"Awan", "Sari Utami is a singer. Awan follows her."},
		{"Awan", "Sari Utami is not known as Awan."},
		{"Awan", "Sari Utami, allegedly known as Awan, is a singer."},
		{"Awan", "Sari Utami, not known as Awan, is a singer."},
		{"Awan", "Sari Utami, known as Awan Fanpage, is a singer."},
		{"Awan", "Sari Utami, known as Awanita, is a singer."},
		{"Awan", "Awan's husband Sari Utami is a writer."},
		{"Awan", "Awan's real name is unknown."},
		{"Awan", "Awan's real name is possibly Sari Utami."},
		{"Awan", "Awan's real name is not Sari Utami."},
		{"Awan", "Awan might be Sari Utami."},
		{"Awan", "Awan, whose real name is Unknown, allegedly lives nearby."},
		{"Awan", "Her friend Sari Utami, known as Awan, is a singer."},
		{"Awan", "If Sari Utami, known as Awan, is a singer, call her."},
		{"Awan", "Fans claim Sari Utami, known as Awan, is a singer."},
		{"Awan", "Nama akun: @sari_utami. Nama panggung: Awan."},
		{"Awan", "Awan and Sari Utami appear together."},
		{"Awan", "Awan is the name of a company founded by Sari Utami."},
		{"Awan", "Sari Utami, formerly employed by Awan, is a singer."},
		{"Awan", "Sari Utami, known for Awan, is a writer."},
		{"", "Sari Utami, known as Awan, is a singer."},
		{"Awan", "Sari Utami, known as Awan, is a fictional character."},
		{"Awan", "Sari Utami, known as Awan, is an Indonesian rock band."},
		{"Awan", "Sari Utami, known as Awan, is a company."},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			if got := Find(tc.subject, tc.text); len(got) != 0 {
				t.Fatalf("unsupported identity claim: %+v", got)
			}
		})
	}
}

func FuzzNamesRetainLiteralEvidence(f *testing.F) {
	f.Add("Awan", "Sari Utami, known as Awan, is a singer.")
	f.Add("Mono", "Sutrisno, dikenal sebagai Mono, adalah pelawak.")
	f.Fuzz(func(t *testing.T, subject, text string) {
		if len(text) > 20000 || len(subject) > 100 {
			t.Skip()
		}
		for _, c := range Find(subject, text) {
			if !strings.Contains(text, c.Quote) || !strings.Contains(c.Quote, c.Name) || !strings.Contains(c.Quote, c.Alias) || !Equal(subject, c.Name) && !Equal(subject, c.Alias) {
				t.Fatalf("ungrounded name claim: %+v", c)
			}
		}
	})
}
