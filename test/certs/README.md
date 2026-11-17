# How to generate key and cert:

## Make private key without a password

certtool --rsa --generate-privkey --null-password --outfile=domain.key

## Use ``domain.cfg`` template to make self-signed cert

certtool --generate-self-signed --load-privkey=domain.key --template=domain.cfg --outfile=domain.crt --load-ca-privkey=domain.key --null-password --no-text
