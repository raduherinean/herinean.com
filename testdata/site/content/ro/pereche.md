---
title: "De ce îmi scriu propriul generator de site static"
date: 2026-09-06
key: pair
pillar: analysis
summary: "Framework-urile rezolvă probleme pe care nu le am și le ascund pe cele pe care le am. Generatorul din spatele acestui site are câteva sute de linii de Go."
---
De fiecare dată când pornesc un site personal, aleg un generator de site static, pierd o după-amiază luptându-mă cu sistemul lui de plugin-uri și ajung cu o construcție pe care nu o înțeleg complet. De data aceasta am decis să scriu generatorul, în loc să aleg unul existent.

Cerințele s-au dovedit a fi mici: să parseze Markdown cu front matter, să valideze câteva câmpuri, să randeze conținut în două limbi și să producă HTML simplu, fără JavaScript pe partea de client. Niciuna dintre acestea nu are nevoie de un ecosistem de plugin-uri sau de un limbaj propriu de template-uri — are nevoie de un parser, câteva structuri de date și teste care eșuează zgomotos când unei piese îi lipsește ceva.

Faptul că îl scriu singur înseamnă și că modurile de eșec îmi aparțin mie, spre reparare. Când o construcție se rupe pentru că lipsește o traducere sau o imagine nu are text alternativ, eroarea indică exact fișierul și linia, pentru că eu am scris codul care produce acel mesaj.

Analiza pe care o voi relua în articolele viitoare urmărește exact aceste decizii: ce câștig și ce pierd renunțând la un instrument gata făcut, în schimbul controlului deplin asupra fiecărui octet publicat.
