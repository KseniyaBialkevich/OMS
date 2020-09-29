sudo docker build -t oms1 .
sudo docker run -p 8080:8080 --net=host --env host=localhost --env port=5432 --env user=kspsql --env password=pass1111 --env dbname=oms_db  oms1:latest 