curl -F "file=@myfile.txt" -F "user=alice" http://localhost:8080/upload


curl "http://localhost:8080/copy?guid=28cd299a-05c5-4bd9-b5a3-69917ac1d832&to_user=bob&subfolder=docs"


curl "http://localhost:8080/list?user=alice"
curl "http://localhost:8080/list?user=bob"


curl -OJ "http://localhost:8080/download?user=alice&path=/myfile.txt"



echo "some text" > myfile.txt
curl -F "file=@myfile.txt" -F "user=alice" http://localhost:8080/upload
rm myfile.txt
curl "http://localhost:8080/list?user=alice"
curl -OJ "http://localhost:8080/download?user=alice&path=myfile.txt"
cat myfile.txt
rm myfile.txt

curl "http://localhost:8080/allow_write?user=bob&allowed=alice"

curl "http://localhost:8080/copy_by_path?from_user=alice&src_path=myfile.txt&to_user=bob&subfolder=shared"
curl "http://localhost:8080/list?user=bob"
curl -OJ "http://localhost:8080/download?user=bob&path=shared/myfile.txt"
cat myfile.txt
rm myfile.txt
