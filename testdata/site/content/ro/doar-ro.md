---
title: "Oportunitatea din spatele unui site fără JavaScript"
date: 2026-09-10
updated: 2026-09-12
key: solo
pillar: opportunity
summary: "Un site care nu rulează JavaScript pe partea de client este mai rapid, mai ușor de auditat și mai greu de spart. Merită efortul suplimentar."
---
Majoritatea site-urilor personale ajung să încarce zeci de scripturi fără ca nimeni să decidă explicit acest lucru: câte un widget de analytics, un font extern, un buton de distribuire pe rețele sociale. Fiecare pare mic, dar suma lor încetinește pagina și mărește suprafața de atac.

Oportunitatea reală nu este doar viteza, deși aceasta contează. Un site fără JavaScript pe partea de client este mult mai ușor de auditat: tot ce se întâmplă este vizibil în sursa paginii, nu depinde de un script terț care se poate schimba fără știrea ta. Nu există stare ascunsă, nu există cereri de rețea neanunțate.

Costul este disciplina de a construi totul la momentul generării paginii, nu în browser. Interactivitatea care ar necesita de obicei JavaScript trebuie fie evitată, fie mutată pe server sau la marginea rețelei, acolo unde poate fi controlată și verificată.

Actualizarea de mai jos reflectă exact acest tip de decizie: am renunțat la un mic widget de căutare pe partea de client în favoarea unei pagini simple, statice, cu toate articolele listate — mai puțin impresionant, dar mult mai ușor de întreținut pe termen lung.
