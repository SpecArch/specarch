# Handbuch der Zweigstelle

Das Handbuch für das Personal einer Zweigstelle. Ausgabe 4.

## 1 Mitglieder

1.1 Ein Mitglied erkennt man an seiner Kartennummer. Die Zweigstelle erfasst
von jedem Mitglied den vollen Namen und die Telefonnummer, die personenbezogen sind.

| Nr. | Feld | Typ | Pflicht | Schutzbedarf |
|---|---|---|---|---|
| 1 | Kartennummer | Text | ja | intern |
| 2 | Voller Name | Text | ja | personenbezogen |

1.2 Das Personal soll die Karte bei jedem Besuch prüfen.

## 2 Ausleihen

2.1 Eine Ausleihe dauert 14 Tage. Sie muss an die Zweigstelle zurückgegeben werden, die sie verliehen hat.

| Feld | Typ | Pflicht |
|---|---|---|
| Ausgeliehen am | Datum | ja |
| Gebuehr | Geld | nein |
| Telefonnummer | Text | nein |

2.2 Die Zweigstelle muss vor dem Fälligkeitstag eine Erinnerung senden:

- per SMS, und
- per Post, wenn das Mitglied kein Telefon hat.

## 3 Öffnung

Die Zweigstelle öffnet um 9.00 Uhr.

| Tag | Stunden |
|---|---|
| Montag | 9.00 bis 17.00 |

```
open 09:00
```
