# Corresponding Source für die laufende 99tools-Record-Version

Vor dem produktiven Docker-Build auf dem Server:

```bash
cd /opt/99tools-record-source
mkdir -p /tmp/99tools-source web/public/source

tar -czf /tmp/99tools-source/99tools-record-source-v1.90.4.tar.gz \
  --exclude='.git' \
  --exclude='web/public/source' \
  --exclude='web-vor-*' \
  --exclude='*.tar.gz' \
  .

cp /tmp/99tools-source/99tools-record-source-v1.90.4.tar.gz \
  web/public/source/99tools-record-source-v1.90.4.tar.gz
```

Danach den Docker-Build starten. Der Link in Einstellungen → Open Source & Lizenzen verweist auf:

`/source/99tools-record-source-v1.90.4.tar.gz`

Wichtig: Bei späteren Änderungen muss das Archiv vor jedem produktiven Build neu erzeugt werden, damit es exakt
zur laufenden modifizierten Version passt.
